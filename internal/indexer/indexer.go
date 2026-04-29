package indexer

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const liveSearchDepth = 4
const indexProgressLogFile = "indexing_progress.log"
const indexProgressLogStep = 100

const (
	entryStateActive  = "active"
	entryStateStale   = "stale"
	entryStateMissing = "missing"
)

type Entry struct {
	Path    string    `json:"path"`
	Name    string    `json:"name"`
	Dir     string    `json:"dir"`
	Ext     string    `json:"ext"`
	IsDir   bool      `json:"isDir"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
}

type Status struct {
	State        string    `json:"state"`
	Roots        []string  `json:"roots"`
	IndexedFiles int64     `json:"indexedFiles"`
	IndexedDirs  int64     `json:"indexedDirs"`
	Errors       int64     `json:"errors"`
	StartedAt    time.Time `json:"startedAt"`
	CompletedAt  time.Time `json:"completedAt"`
}

type Manager struct {
	roots []string
	store string

	entriesMu sync.Mutex
	entries   []indexedEntry

	statusMu sync.RWMutex
	status   Status

	scanMu     sync.Mutex
	cancelScan context.CancelFunc
	scanID     atomic.Uint64

	indexedFiles atomic.Int64
	indexedDirs  atomic.Int64
	errorCount   atomic.Int64

	discoveredItems      atomic.Int64
	progressLoggedItems  atomic.Int64
	progressLogWriteLock sync.Mutex
}

type indexedEntry struct {
	Entry
	nameLower string
	pathLower string
	state     string
	seenScan  uint64
}

type scoredEntry struct {
	Entry
	score int
}

func NewManager(roots []string) *Manager {
	manager := &Manager{
		roots: append([]string(nil), roots...),
		store: defaultIndexStorePath(),
	}
	manager.loadPersistedIndex()
	return manager
}

func (m *Manager) Start() {
	m.startScan()
}

func (m *Manager) Rescan() {
	m.startScan()
}

func (m *Manager) Search(request SearchRequest) []Entry {
	search := compileSearchRequest(request)
	if search.query == "" {
		return nil
	}

	entries := m.snapshotEntries()
	if len(entries) == 0 {
		return m.searchLive(search)
	}

	workers := runtime.GOMAXPROCS(0)
	if workers < 1 {
		workers = 1
	}
	if len(entries) < workers*750 {
		workers = max(1, len(entries)/750)
	}
	if workers < 1 {
		workers = 1
	}

	chunkSize := (len(entries) + workers - 1) / workers
	results := make(chan []scoredEntry, workers)
	var wg sync.WaitGroup

	for start := 0; start < len(entries); start += chunkSize {
		end := min(start+chunkSize, len(entries))
		wg.Add(1)
		go func(chunk []indexedEntry) {
			defer wg.Done()
			local := make([]scoredEntry, 0, search.limit)
			for _, item := range chunk {
				if !search.matches(item.Entry) {
					continue
				}

				score := scoreEntry(search.query, item)
				if score <= 0 {
					continue
				}

				local = appendScored(local, scoredEntry{
					Entry: item.Entry,
					score: score,
				}, search.limit)
			}
			results <- local
		}(entries[start:end])
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	merged := make([]scoredEntry, 0, search.limit)
	for batch := range results {
		for _, item := range batch {
			merged = appendScored(merged, item, search.limit)
		}
	}

	out := make([]Entry, 0, len(merged))
	for _, item := range merged {
		out = append(out, item.Entry)
	}

	status := m.Status()
	if len(out) == 0 && status.State != "scanning" {
		out = mergeEntries(out, m.searchLive(search), search.limit)
	}

	out = uniqueEntries(out, search.limit)
	out = m.hydrateAndPruneMissing(out)
	return out
}

func (m *Manager) Status() Status {
	m.statusMu.RLock()
	status := m.status
	m.statusMu.RUnlock()

	status.IndexedFiles = m.indexedFiles.Load()
	status.IndexedDirs = m.indexedDirs.Load()
	status.Errors = m.errorCount.Load()
	status.Roots = append([]string(nil), status.Roots...)
	return status
}

func (m *Manager) startScan() {
	m.scanMu.Lock()
	defer m.scanMu.Unlock()

	if m.cancelScan != nil {
		m.cancelScan()
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.cancelScan = cancel

	scanID := m.scanID.Add(1)

	m.indexedFiles.Store(0)
	m.indexedDirs.Store(0)
	m.errorCount.Store(0)
	m.discoveredItems.Store(0)
	m.progressLoggedItems.Store(0)
	m.resetProgressLog()

	now := time.Now()
	m.statusMu.Lock()
	m.status = Status{
		State:       "scanning",
		Roots:       append([]string(nil), m.roots...),
		StartedAt:   now,
		CompletedAt: time.Time{},
	}
	m.statusMu.Unlock()

	go m.scan(ctx, scanID)
}

func (m *Manager) scan(ctx context.Context, scanID uint64) {
	workerCount := max(runtime.GOMAXPROCS(0)*4, 8)
	dirs := newScanDirQueue(ctx)
	var workers sync.WaitGroup
	var visited sync.Map

	enqueue := func(path string) bool {
		if ctx.Err() != nil {
			return false
		}

		path = filepath.Clean(path)
		key := strings.ToLower(path)
		if _, exists := visited.LoadOrStore(key, struct{}{}); exists {
			return false
		}

		return dirs.push(path)
	}

	for _, root := range m.roots {
		enqueue(filepath.Clean(root))
	}

	for i := 0; i < workerCount; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()

			batch := make([]indexedEntry, 0, 256)
			flush := func() {
				if len(batch) == 0 {
					return
				}
				m.appendBatch(scanID, batch)
				batch = make([]indexedEntry, 0, 256)
			}
			defer flush()

			for {
				dir, ok := dirs.pop()
				if !ok {
					return
				}

				if ctx.Err() != nil {
					dirs.done()
					continue
				}

				dirEntries, err := os.ReadDir(dir)
				if err != nil {
					m.errorCount.Add(1)
					dirs.done()
					continue
				}

				for _, entry := range dirEntries {
					if ctx.Err() != nil {
						break
					}

					name := entry.Name()
					fullPath := filepath.Join(dir, name)
					isDir := entry.IsDir()

					item := indexedEntry{
						Entry: Entry{
							Path:  fullPath,
							Name:  name,
							Dir:   dir,
							Ext:   strings.ToLower(filepath.Ext(name)),
							IsDir: isDir,
						},
						nameLower: normalize(name),
						pathLower: normalize(fullPath),
						state:     entryStateActive,
						seenScan:  scanID,
					}

					discovered := m.discoveredItems.Add(1)
					m.logIndexProgress(scanID, discovered, false)

					batch = append(batch, item)
					if len(batch) >= cap(batch) {
						flush()
					}

					if isDir {
						if entry.Type()&os.ModeSymlink != 0 {
							continue
						}
						m.indexedDirs.Add(1)
						enqueue(fullPath)
						continue
					}

					m.indexedFiles.Add(1)
				}

				dirs.done()
			}
		}()
	}

	workers.Wait()
	dirs.close()

	m.statusMu.Lock()

	if scanID != m.scanID.Load() {
		m.statusMu.Unlock()
		return
	}

	if ctx.Err() != nil {
		m.status.State = "cancelled"
		m.status.CompletedAt = time.Now()
		m.statusMu.Unlock()
		m.logIndexProgress(scanID, m.discoveredItems.Load(), true)
		return
	}

	m.status.State = "ready"
	m.status.CompletedAt = time.Now()
	m.statusMu.Unlock()

	m.logIndexProgress(scanID, m.discoveredItems.Load(), true)
	m.compactAndPersistIndex()
}

type scanDirQueue struct {
	ctx    context.Context
	mu     sync.Mutex
	cond   *sync.Cond
	jobs   []string
	head   int
	active int
	closed bool
	stop   chan struct{}
	once   sync.Once
}

func newScanDirQueue(ctx context.Context) *scanDirQueue {
	q := &scanDirQueue{
		ctx:  ctx,
		jobs: make([]string, 0, 1024),
		stop: make(chan struct{}),
	}
	q.cond = sync.NewCond(&q.mu)

	go func() {
		select {
		case <-ctx.Done():
			q.close()
		case <-q.stop:
		}
	}()

	return q
}

func (q *scanDirQueue) push(path string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed || q.ctx.Err() != nil {
		return false
	}

	q.jobs = append(q.jobs, path)
	q.cond.Signal()
	return true
}

func (q *scanDirQueue) pop() (string, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for {
		if q.closed || q.ctx.Err() != nil {
			return "", false
		}

		if q.head < len(q.jobs) {
			path := q.jobs[q.head]
			q.jobs[q.head] = ""
			q.head++
			q.active++

			if q.head > 4096 && q.head*2 >= len(q.jobs) {
				q.jobs = append([]string(nil), q.jobs[q.head:]...)
				q.head = 0
			}

			return path, true
		}

		if q.active == 0 {
			q.closed = true
			q.cond.Broadcast()
			return "", false
		}

		q.cond.Wait()
	}
}

func (q *scanDirQueue) done() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.active > 0 {
		q.active--
	}
	if q.active == 0 && q.head >= len(q.jobs) {
		q.closed = true
		q.cond.Broadcast()
		return
	}
	q.cond.Signal()
}

func (q *scanDirQueue) close() {
	q.once.Do(func() {
		close(q.stop)
		q.mu.Lock()
		q.closed = true
		q.cond.Broadcast()
		q.mu.Unlock()
	})
}

func defaultIndexStorePath() string {
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		return "better-search-index.db"
	}
	return filepath.Join(base, "better-search", "index.db")
}

func (m *Manager) loadPersistedIndex() {
	if m.store == "" {
		return
	}

	entries, lastScan, err := loadSQLiteIndex(m.store)
	if err != nil {
		return
	}
	m.scanID.Store(lastScan)

	m.entriesMu.Lock()
	m.entries = entries
	m.entriesMu.Unlock()
}

func (m *Manager) compactAndPersistIndex() {
	scanID := m.scanID.Load()

	m.entriesMu.Lock()
	latest := make(map[string]indexedEntry, len(m.entries))
	for _, entry := range m.entries {
		key := strings.ToLower(filepath.Clean(entry.Path))
		current, exists := latest[key]
		if !exists || entry.seenScan >= current.seenScan {
			latest[key] = entry
		}
	}

	compact := make([]indexedEntry, 0, len(latest))
	for _, entry := range latest {
		if entry.seenScan == scanID {
			entry.state = entryStateActive
		} else if entry.state == "" || entry.state == entryStateActive {
			entry.state = entryStateStale
		}
		compact = append(compact, entry)
	}
	sort.Slice(compact, func(i, j int) bool {
		return strings.ToLower(compact[i].Path) < strings.ToLower(compact[j].Path)
	})
	m.entries = compact
	m.entriesMu.Unlock()

	if m.store == "" {
		return
	}

	_ = saveSQLiteIndex(m.store, compact, scanID)
}

func (m *Manager) resetProgressLog() {
	_ = os.WriteFile(indexProgressLogFile, nil, 0o644)
}

func (m *Manager) logIndexProgress(scanID uint64, count int64, force bool) {
	if scanID != m.scanID.Load() || count <= 0 {
		return
	}
	if !force && count%indexProgressLogStep != 0 {
		return
	}

	m.progressLogWriteLock.Lock()
	defer m.progressLogWriteLock.Unlock()

	if scanID != m.scanID.Load() || count <= m.progressLoggedItems.Load() {
		return
	}

	line := time.Now().Format(time.RFC3339) + ": " + strconv.FormatInt(count, 10) + "\n"
	file, err := os.OpenFile(indexProgressLogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer file.Close()

	if _, err := file.WriteString(line); err != nil {
		return
	}
	m.progressLoggedItems.Store(count)
}

func (m *Manager) appendBatch(scanID uint64, batch []indexedEntry) {
	if scanID != m.scanID.Load() {
		return
	}

	copyBatch := append([]indexedEntry(nil), batch...)
	m.entriesMu.Lock()
	m.entries = append(m.entries, copyBatch...)
	m.entriesMu.Unlock()
}

func (m *Manager) snapshotEntries() []indexedEntry {
	m.entriesMu.Lock()
	entries := append([]indexedEntry(nil), m.entries...)
	m.entriesMu.Unlock()
	return entries
}

func (m *Manager) searchLive(search compiledSearch) []Entry {
	type liveDir struct {
		path  string
		depth int
	}

	queue := make([]liveDir, 0, len(m.roots)*4)
	visited := make(map[string]struct{}, len(m.roots)*8)
	enqueue := func(path string, depth int) bool {
		path = filepath.Clean(path)
		key := strings.ToLower(path)
		if _, exists := visited[key]; exists {
			return false
		}
		visited[key] = struct{}{}
		queue = append(queue, liveDir{path: path, depth: depth})
		return true
	}

	for _, root := range m.roots {
		enqueue(root, 0)
	}

	merged := make([]scoredEntry, 0, search.limit)
	for i := 0; i < len(queue); i++ {
		item := queue[i]
		dirEntries, err := os.ReadDir(item.path)
		if err != nil {
			continue
		}

		for _, entry := range dirEntries {
			name := entry.Name()
			fullPath := filepath.Join(item.path, name)
			isDir := entry.IsDir()

			info, infoErr := entry.Info()
			if infoErr != nil {
				continue
			}

			candidate := indexedEntry{
				Entry: Entry{
					Path:    fullPath,
					Name:    name,
					Dir:     item.path,
					Ext:     strings.ToLower(filepath.Ext(name)),
					IsDir:   isDir,
					Size:    info.Size(),
					ModTime: info.ModTime(),
				},
				nameLower: normalize(name),
				pathLower: normalize(fullPath),
			}

			if isDir && item.depth < liveSearchDepth && entry.Type()&os.ModeSymlink == 0 {
				enqueue(fullPath, item.depth+1)
			}

			if !search.matches(candidate.Entry) {
				continue
			}

			score := scoreEntry(search.query, candidate)
			if score > 0 {
				merged = appendScored(merged, scoredEntry{
					Entry: candidate.Entry,
					score: score + 200,
				}, search.limit)
			}

		}
	}

	out := make([]Entry, 0, len(merged))
	for _, item := range merged {
		out = append(out, item.Entry)
	}

	return out
}

func mergeEntries(primary, secondary []Entry, limit int) []Entry {
	if limit <= 0 {
		limit = 50
	}

	seen := make(map[string]struct{}, limit)
	out := make([]Entry, 0, min(limit, len(primary)+len(secondary)))

	appendUnique := func(entries []Entry) {
		for _, entry := range entries {
			key := strings.ToLower(entry.Path)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, entry)
			if len(out) >= limit {
				return
			}
		}
	}

	appendUnique(primary)
	if len(out) < limit {
		appendUnique(secondary)
	}

	return out
}

func uniqueEntries(entries []Entry, limit int) []Entry {
	if limit <= 0 {
		limit = 50
	}

	seen := make(map[string]struct{}, min(limit, len(entries)))
	out := make([]Entry, 0, min(limit, len(entries)))
	for _, entry := range entries {
		key := strings.ToLower(filepath.Clean(entry.Path))
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, entry)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func (m *Manager) hydrateAndPruneMissing(entries []Entry) []Entry {
	missingPaths := make([]string, 0)
	out := entries[:0]

	for i := range entries {
		info, err := os.Stat(entries[i].Path)
		if err != nil {
			if os.IsNotExist(err) {
				missingPaths = append(missingPaths, entries[i].Path)
				continue
			}
			out = append(out, entries[i])
			continue
		}
		entries[i].IsDir = info.IsDir()
		entries[i].Size = info.Size()
		entries[i].ModTime = info.ModTime()
		out = append(out, entries[i])
	}

	if len(missingPaths) > 0 {
		m.removeIndexedPaths(missingPaths)
		if m.store != "" {
			_ = markSQLitePathsState(m.store, missingPaths, entryStateMissing)
		}
	}

	return out
}

func (m *Manager) removeIndexedPaths(paths []string) {
	missing := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		missing[strings.ToLower(filepath.Clean(path))] = struct{}{}
	}

	m.entriesMu.Lock()
	defer m.entriesMu.Unlock()

	kept := m.entries[:0]
	for _, entry := range m.entries {
		key := strings.ToLower(filepath.Clean(entry.Path))
		if _, remove := missing[key]; remove {
			continue
		}
		kept = append(kept, entry)
	}
	m.entries = kept
}

func appendScored(items []scoredEntry, candidate scoredEntry, limit int) []scoredEntry {
	items = append(items, candidate)
	sort.Slice(items, func(i, j int) bool {
		if items[i].score == items[j].score {
			if items[i].ModTime.Equal(items[j].ModTime) {
				return len(items[i].Path) < len(items[j].Path)
			}
			return items[i].ModTime.After(items[j].ModTime)
		}
		return items[i].score > items[j].score
	})

	if len(items) > limit {
		items = items[:limit]
	}

	return items
}

func scoreEntry(query string, item indexedEntry) int {
	tokens := strings.Fields(query)
	if len(tokens) == 0 {
		return 0
	}

	total := 0
	for _, token := range tokens {
		best := scoreToken(token, item.nameLower, item.pathLower, item.Ext, item.IsDir)
		if best == 0 {
			return 0
		}
		total += best
	}

	if item.IsDir {
		total += 25
	}
	if strings.HasPrefix(item.Ext, "."+query) || item.Ext == query {
		total += 60
	}
	return total
}

func scoreToken(token, name, path, ext string, isDir bool) int {
	switch {
	case name == token:
		return 1600
	case strings.HasPrefix(name, token):
		return 1400 - min(len(name), 180)
	case token == ext:
		return 1320
	}

	if idx := strings.Index(name, token); idx >= 0 {
		return 1180 - idx*18
	}
	if idx := strings.Index(path, token); idx >= 0 {
		return 920 - idx
	}

	if subseq := subsequenceScore(name, token); subseq > 0 {
		return 650 + subseq
	}
	if subseq := subsequenceScore(path, token); subseq > 0 {
		return 420 + subseq
	}
	if isDir && strings.Contains(name, strings.TrimSuffix(token, `\`)) {
		return 260
	}

	return 0
}

func subsequenceScore(haystack, needle string) int {
	if needle == "" {
		return 0
	}

	haystackRunes := []rune(haystack)
	needleRunes := []rune(needle)
	index := 0
	lastMatch := -2
	streak := 0
	score := 0

	for pos, r := range haystackRunes {
		if index >= len(needleRunes) {
			break
		}

		if r != needleRunes[index] {
			if streak > 0 {
				streak = 0
			}
			continue
		}

		score += 8
		if lastMatch >= 0 && lastMatch+1 == pos {
			streak++
			score += streak * 4
		} else {
			streak = 1
		}

		lastMatch = pos
		index++
	}

	if index != len(needleRunes) {
		return 0
	}

	return score
}

func normalize(value string) string {
	value = strings.ToLower(value)
	replacer := strings.NewReplacer(`\`, " ", "/", " ", "-", " ", "_", " ")
	value = replacer.Replace(value)
	return strings.Join(strings.Fields(value), " ")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

package indexer

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const liveSearchDepth = 2

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
}

type indexedEntry struct {
	Entry
	nameLower string
	pathLower string
}

type scoredEntry struct {
	Entry
	score int
}

func NewManager(roots []string) *Manager {
	return &Manager{
		roots: append([]string(nil), roots...),
	}
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
	if status.State != "ready" || len(out) == 0 {
		out = mergeEntries(out, m.searchLive(search), search.limit)
	}

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
	m.entriesMu.Lock()
	m.entries = nil
	m.entriesMu.Unlock()

	m.indexedFiles.Store(0)
	m.indexedDirs.Store(0)
	m.errorCount.Store(0)

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
	dirs := make(chan string, workerCount*4)
	var pending sync.WaitGroup
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

		pending.Add(1)
		select {
		case dirs <- path:
			return true
		case <-ctx.Done():
			pending.Done()
			return false
		}
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

			for dir := range dirs {
				if ctx.Err() != nil {
					pending.Done()
					continue
				}

				dirEntries, err := os.ReadDir(dir)
				if err != nil {
					m.errorCount.Add(1)
					pending.Done()
					continue
				}

				for _, entry := range dirEntries {
					if ctx.Err() != nil {
						break
					}

					name := entry.Name()
					fullPath := filepath.Join(dir, name)
					isDir := entry.IsDir()

					info, infoErr := entry.Info()
					if infoErr != nil {
						m.errorCount.Add(1)
						continue
					}

					item := indexedEntry{
						Entry: Entry{
							Path:    fullPath,
							Name:    name,
							Dir:     dir,
							Ext:     strings.ToLower(filepath.Ext(name)),
							IsDir:   isDir,
							Size:    info.Size(),
							ModTime: info.ModTime(),
						},
						nameLower: normalize(name),
						pathLower: normalize(fullPath),
					}

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

				pending.Done()
			}

			flush()
		}()
	}

	for _, root := range m.roots {
		enqueue(filepath.Clean(root))
	}

	go func() {
		pending.Wait()
		close(dirs)
	}()

	workers.Wait()

	m.statusMu.Lock()
	defer m.statusMu.Unlock()

	if scanID != m.scanID.Load() {
		return
	}

	if ctx.Err() != nil {
		m.status.State = "cancelled"
		m.status.CompletedAt = time.Now()
		return
	}

	m.status.State = "ready"
	m.status.CompletedAt = time.Now()
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
	entries := m.entries
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

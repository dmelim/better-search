# Search Index Improvement Plan

## Goal

Make search return useful results immediately, keep the index fresh in the background, and avoid situations where known files disappear from search just because indexing has not reached them yet.

The guiding rule is: search should never wait for crawling. Search should query the best known index immediately, while background work improves that index over time.

## Current State

- The app recursively scans configured roots and keeps entries in memory.
- A persisted JSON index has been introduced so startup can search the previous run's known paths.
- The current scanner streams discovered entries into the in-memory index while scanning.
- Search validates visible results with `os.Stat` and removes paths that no longer exist.
- The frontend refreshes the active query while indexing is running.

This is a good first step, but it is still a prototype indexing model. The next work should make persistence safer, stale data handling more careful, and refresh behavior more intentional.

## Target Architecture

Use a layered index:

```text
stable persisted index
        +
current scan overlay
        +
dirty folder refreshes
        =
searchable view
```

Search reads from the combined searchable view. Scanning, validation, and filesystem events update that view asynchronously.

## Phase 1: Harden Persistence

### 1. Atomic Writes

Do not write directly over the saved index.

Write flow:

```text
index.tmp
close file
rename index.tmp -> index.json
```

This prevents a crash or forced app exit from leaving a corrupt index file.

### 2. Versioned Index File

Wrap saved entries in a schema object:

```json
{
  "version": 1,
  "savedAt": "2026-04-26T12:00:00+01:00",
  "roots": ["C:\\"],
  "entries": []
}
```

This gives us room to change the storage format without guessing what an old file contains.

### 3. Save Debouncing

Avoid saving after every small change.

Save when:

- a full scan completes,
- a large batch threshold is crossed,
- the app is shutting down, if a clean shutdown hook is available.

## Phase 2: Safer Stale Handling

### 1. Add Entry State

Track state instead of immediately deleting entries:

```text
active
stale
missing
inaccessible
```

Only hide `missing` entries from normal results. Keep `inaccessible` entries because permission errors do not mean the file was deleted.

### 2. Generation-Based Scans

Each scan gets a generation ID.

```text
entry.last_seen_scan = current_scan_id
```

When a scan completes, entries under scanned roots that were not seen can be marked stale. After a second confirmation or a successful full scan, stale entries can become missing.

### 3. Validate With Budgets

Validate only the visible result set during search.

Rules:

- validate first page of results first,
- cap validation work per search,
- mark uncertain results stale instead of blocking the UI.

## Phase 3: Improve Scanner Scheduling

### 1. Keep Search Independent

Search should always return from the current index snapshot. It should not trigger an expensive filesystem crawl directly.

### 2. Priority Scan Queue

Use priorities for directories:

```text
high: current query hints and selected scope
medium: user folders, recent folders, unfinished folders
low: broad drive crawl
```

If the user searches for `rom`, boost directories whose path or name contains likely hints such as `rom`, `games`, or recently matched parent folders.

### 3. Scan Continuation

Persist unfinished scan state:

```text
root
folder path
last scanned time
priority
```

On next startup, resume important unfinished areas before doing a broad full scan.

## Phase 4: Move Storage Beyond JSON

JSON is acceptable for the prototype, but it will become slow and memory-heavy for large drives.

Preferred options:

### SQLite

Recommended if we want queryability and durability.

Candidate schema:

```sql
entries(
  id INTEGER PRIMARY KEY,
  parent_id INTEGER,
  path TEXT UNIQUE,
  name TEXT,
  name_lower TEXT,
  ext TEXT,
  is_dir INTEGER,
  size INTEGER,
  mod_time INTEGER,
  state TEXT,
  last_seen_scan INTEGER
);

CREATE INDEX entries_name_lower_idx ON entries(name_lower);
CREATE INDEX entries_ext_idx ON entries(ext);
CREATE INDEX entries_parent_idx ON entries(parent_id);
CREATE INDEX entries_state_idx ON entries(state);
```

### bbolt

Good if we want a simple embedded key-value store and keep search indexing mostly in memory.

## Phase 5: Filesystem Change Tracking

### 1. Watcher Dirty Queue

Use filesystem events to mark folders dirty, not as the only source of truth.

Flow:

```text
filesystem event
mark folder dirty
debounce
rescan dirty folder
update index
```

If a watcher overflows or errors, mark the affected subtree dirty and rescan it.

### 2. Avoid Watching Everything Blindly

Watching every directory can be expensive and fragile. Start with configured roots and important subtrees. Expand only when needed.

### 3. Consider USN Journal Later

For NTFS volumes, the USN Change Journal is the stronger long-term solution. It can detect changes that happened while the app was closed. It is more complex, so it should come after the persisted index and stale-state model are solid.

## Phase 6: Result Quality

### 1. Preserve Ranking

Persist enough fields to rank without rebuilding full metadata:

- normalized name,
- normalized path,
- extension,
- directory flag,
- last opened or selected time later.

### 2. Prefer Confirmed Results

Ranking should slightly prefer:

- active entries over stale entries,
- recently verified entries,
- exact name matches,
- shorter paths for equal scores.

### 3. UI Feedback

Show indexing state without blocking results:

```text
25 matches from saved index
Indexing continues...
```

Avoid full-screen loading states once any searchable index exists.

## Implementation Order

1. Add atomic persisted index writes.
2. Add index file version wrapper.
3. Add entry state and stop hard-deleting on first failed `os.Stat`.
4. Add scan generation IDs and stale marking.
5. Add validation budget for visible results.
6. Add priority scan queue based on current query and selected scope.
7. Move persistence to SQLite or bbolt.
8. Add watcher dirty queue.
9. Investigate USN Journal for NTFS volumes.

## Non-Goals For Now

- Do not build full content search.
- Do not rely on Windows Search as the primary index.
- Do not require Everything to be installed.
- Do not add network-drive-specific behavior until local indexing is solid.

## Success Criteria

- Search returns previous known results immediately after startup.
- New results appear progressively during indexing.
- A missing file is hidden without destroying unrelated cached data.
- Interrupted scans do not corrupt the persisted index.
- Broad drive indexing does not block active search.
- The app can explain whether results are from saved data, fresh scan data, or recently verified paths.

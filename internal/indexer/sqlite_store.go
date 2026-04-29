package indexer

import (
	"database/sql"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const sqliteSchema = `
PRAGMA journal_mode=WAL;
PRAGMA synchronous=NORMAL;

CREATE TABLE IF NOT EXISTS meta (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS entries (
	path TEXT PRIMARY KEY,
	dir TEXT NOT NULL,
	name TEXT NOT NULL,
	name_lower TEXT NOT NULL,
	path_lower TEXT NOT NULL,
	ext TEXT NOT NULL,
	is_dir INTEGER NOT NULL,
	size INTEGER NOT NULL,
	mod_time INTEGER NOT NULL,
	state TEXT NOT NULL,
	last_seen_scan INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS entries_name_lower_idx ON entries(name_lower);
CREATE INDEX IF NOT EXISTS entries_ext_idx ON entries(ext);
CREATE INDEX IF NOT EXISTS entries_state_idx ON entries(state);
CREATE INDEX IF NOT EXISTS entries_last_seen_scan_idx ON entries(last_seen_scan);
`

func openSQLiteIndex(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(sqliteSchema); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func loadSQLiteIndex(path string) ([]indexedEntry, uint64, error) {
	db, err := openSQLiteIndex(path)
	if err != nil {
		return nil, 0, err
	}
	defer db.Close()

	lastScan := loadLastScan(db)
	rows, err := db.Query(`
		SELECT path, dir, name, ext, is_dir, size, mod_time, state, last_seen_scan
		FROM entries
		WHERE state != ?
	`, entryStateMissing)
	if err != nil {
		return nil, lastScan, err
	}
	defer rows.Close()

	entries := make([]indexedEntry, 0, 4096)
	for rows.Next() {
		var entry Entry
		var isDir int
		var modTime int64
		var state string
		var seenScan uint64

		if err := rows.Scan(
			&entry.Path,
			&entry.Dir,
			&entry.Name,
			&entry.Ext,
			&isDir,
			&entry.Size,
			&modTime,
			&state,
			&seenScan,
		); err != nil {
			return entries, lastScan, err
		}

		if entry.Path == "" || entry.Name == "" {
			continue
		}
		if entry.Dir == "" {
			entry.Dir = filepath.Dir(entry.Path)
		}
		if entry.Ext == "" {
			entry.Ext = strings.ToLower(filepath.Ext(entry.Name))
		}

		entry.IsDir = isDir != 0
		if modTime > 0 {
			entry.ModTime = time.Unix(0, modTime)
		}

		entries = append(entries, indexedEntry{
			Entry:     entry,
			nameLower: normalize(entry.Name),
			pathLower: normalize(entry.Path),
			state:     state,
			seenScan:  seenScan,
		})
	}

	return entries, lastScan, rows.Err()
}

func loadLastScan(db *sql.DB) uint64 {
	var value string
	err := db.QueryRow(`SELECT value FROM meta WHERE key = 'last_scan'`).Scan(&value)
	if err != nil {
		return 0
	}

	lastScan, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0
	}
	return lastScan
}

func saveSQLiteIndex(path string, entries []indexedEntry, scanID uint64) error {
	db, err := openSQLiteIndex(path)
	if err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		INSERT INTO meta(key, value) VALUES('version', '1')
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`); err != nil {
		return err
	}

	if _, err := tx.Exec(`
		INSERT INTO meta(key, value) VALUES('saved_at', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, time.Now().Format(time.RFC3339)); err != nil {
		return err
	}

	if _, err := tx.Exec(`
		INSERT INTO meta(key, value) VALUES('last_scan', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, strconv.FormatUint(scanID, 10)); err != nil {
		return err
	}

	stmt, err := tx.Prepare(`
		INSERT INTO entries(
			path, dir, name, name_lower, path_lower, ext, is_dir, size, mod_time, state, last_seen_scan
		)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			dir = excluded.dir,
			name = excluded.name,
			name_lower = excluded.name_lower,
			path_lower = excluded.path_lower,
			ext = excluded.ext,
			is_dir = excluded.is_dir,
			size = excluded.size,
			mod_time = excluded.mod_time,
			state = excluded.state,
			last_seen_scan = excluded.last_seen_scan
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, item := range entries {
		state := item.state
		if state == "" {
			state = entryStateStale
		}

		seenScan := item.seenScan
		if seenScan == 0 && state == entryStateActive {
			seenScan = scanID
		}

		_, err := stmt.Exec(
			item.Path,
			item.Dir,
			item.Name,
			item.nameLower,
			item.pathLower,
			item.Ext,
			boolToInt(item.IsDir),
			item.Size,
			timeToUnixNano(item.ModTime),
			state,
			seenScan,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func markSQLitePathsState(path string, paths []string, state string) error {
	if len(paths) == 0 {
		return nil
	}

	db, err := openSQLiteIndex(path)
	if err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`UPDATE entries SET state = ? WHERE path = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, item := range paths {
		if _, err := stmt.Exec(state, filepath.Clean(item)); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func timeToUnixNano(value time.Time) int64 {
	if value.IsZero() {
		return 0
	}
	return value.UnixNano()
}

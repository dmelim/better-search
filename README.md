# better-search

`better-search` is a Windows-first local file finder written in Go and packaged as a Wails desktop app.

It scans fixed drives concurrently, keeps an in-memory index, and gives you a native desktop window with:

- search by file name, folder name, extension, or path fragment
- open files and folders directly from results
- reveal files in Windows Explorer
- rescan the index without restarting the app

## Run

```powershell
wails dev
```

For a production build:

```powershell
wails build
```

## Current behavior

- roots default to detected fixed drives on Windows
- indexing starts immediately on app startup
- search works against the current in-memory index while scanning continues
- results are ranked with exact, prefix, substring, and fuzzy subsequence matching
- result rows can open files directly or reveal them in Explorer

## Next obvious upgrades

- persist the index to disk between runs
- watch for filesystem changes instead of rescanning everything
- add filters for root, extension, and modified time
- add ignore rules for heavy folders like `node_modules` and `.git`

package indexer

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestSearchAppliesFilters(t *testing.T) {
	temp := t.TempDir()
	docs := filepath.Join(temp, "docs")
	docsArchive := filepath.Join(temp, "docs-archive")
	reportPDF := filepath.Join(docs, "report.PDF")
	reportTXT := filepath.Join(docsArchive, "report.txt")
	reportsDir := filepath.Join(docs, "reports")
	docsDir := docs
	docsArchiveDir := docsArchive

	manager := &Manager{}
	manager.entries = []indexedEntry{
		testIndexedEntry(Entry{Path: reportPDF, Name: "report.PDF", Dir: docs, Ext: ".pdf"}),
		testIndexedEntry(Entry{Path: reportTXT, Name: "report.txt", Dir: docsArchive, Ext: ".txt"}),
		testIndexedEntry(Entry{Path: reportsDir, Name: "reports", Dir: docs, IsDir: true}),
		testIndexedEntry(Entry{Path: docsDir, Name: "docs", Dir: temp, IsDir: true}),
		testIndexedEntry(Entry{Path: docsArchiveDir, Name: "docs-archive", Dir: temp, IsDir: true}),
	}
	manager.status = Status{State: "ready"}

	tests := []struct {
		name    string
		request SearchRequest
		want    []string
	}{
		{
			name: "files only excludes folders",
			request: SearchRequest{
				Query:      "report",
				TypeFilter: "files",
			},
			want: []string{reportPDF, reportTXT},
		},
		{
			name: "folders only excludes files",
			request: SearchRequest{
				Query:      "report",
				TypeFilter: "folders",
			},
			want: []string{reportsDir},
		},
		{
			name: "folder prefix keeps descendants but not sibling prefixes",
			request: SearchRequest{
				Query:        "report",
				FolderPrefix: docs,
			},
			want: []string{reportPDF, reportsDir},
		},
		{
			name: "folder prefix matches exact folder path",
			request: SearchRequest{
				Query:        "docs",
				TypeFilter:   "folders",
				FolderPrefix: docs,
			},
			want: []string{docsDir},
		},
		{
			name: "extension filter is case insensitive without dot",
			request: SearchRequest{
				Query:     "report",
				Extension: "PDF",
			},
			want: []string{reportPDF},
		},
		{
			name: "extension filter accepts leading dot",
			request: SearchRequest{
				Query:     "report",
				Extension: ".pdf",
			},
			want: []string{reportPDF},
		},
		{
			name: "empty query stays empty even with filters",
			request: SearchRequest{
				Query:        "",
				TypeFilter:   "files",
				FolderPrefix: docs,
				Extension:    "pdf",
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := manager.Search(tt.request)
			assertPaths(t, got, tt.want)
		})
	}
}

func TestSearchLiveRespectsFilters(t *testing.T) {
	temp := t.TempDir()
	docs := filepath.Join(temp, "docs")
	docsArchive := filepath.Join(temp, "docs-archive")

	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	if err := os.MkdirAll(docsArchive, 0o755); err != nil {
		t.Fatalf("mkdir archive: %v", err)
	}

	reportPDF := filepath.Join(docs, "report.pdf")
	reportTXT := filepath.Join(docsArchive, "report.txt")

	if err := os.WriteFile(reportPDF, []byte("pdf"), 0o644); err != nil {
		t.Fatalf("write report pdf: %v", err)
	}
	if err := os.WriteFile(reportTXT, []byte("txt"), 0o644); err != nil {
		t.Fatalf("write report txt: %v", err)
	}

	manager := NewManager([]string{temp})
	got := manager.Search(SearchRequest{
		Query:        "report",
		Limit:        10,
		TypeFilter:   "files",
		FolderPrefix: docs,
		Extension:    "pdf",
	})

	assertPaths(t, got, []string{reportPDF})
}

func testIndexedEntry(entry Entry) indexedEntry {
	return indexedEntry{
		Entry:     entry,
		nameLower: normalize(entry.Name),
		pathLower: normalize(entry.Path),
	}
}

func assertPaths(t *testing.T, got []Entry, want []string) {
	t.Helper()

	gotPaths := make([]string, 0, len(got))
	for _, entry := range got {
		gotPaths = append(gotPaths, entry.Path)
	}

	sort.Strings(gotPaths)
	sort.Strings(want)

	if len(gotPaths) != len(want) {
		t.Fatalf("got %v paths, want %v: %v vs %v", len(gotPaths), len(want), gotPaths, want)
	}

	for i := range gotPaths {
		if gotPaths[i] != want[i] {
			t.Fatalf("got paths %v, want %v", gotPaths, want)
		}
	}
}

package indexer

import (
	"os"
	"path/filepath"
	"strings"
)

type SearchRequest struct {
	Query        string `json:"query"`
	Limit        int    `json:"limit"`
	TypeFilter   string `json:"typeFilter"`
	FolderPrefix string `json:"folderPrefix"`
	Extension    string `json:"extension"`
}

type compiledSearch struct {
	query                 string
	limit                 int
	typeFilter            string
	folderPrefix          string
	folderPrefixLower     string
	folderPrefixWithSepLC string
	extension             string
}

func compileSearchRequest(request SearchRequest) compiledSearch {
	limit := request.Limit
	if limit <= 0 {
		limit = 50
	}

	typeFilter := strings.ToLower(strings.TrimSpace(request.TypeFilter))
	switch typeFilter {
	case "files", "folders":
	default:
		typeFilter = "all"
	}

	folderPrefix := cleanSearchPath(request.FolderPrefix)
	folderPrefixLower := strings.ToLower(folderPrefix)
	folderPrefixWithSep := folderPrefix
	if folderPrefixWithSep != "" && !strings.HasSuffix(folderPrefixWithSep, string(os.PathSeparator)) {
		folderPrefixWithSep += string(os.PathSeparator)
	}

	return compiledSearch{
		query:                 normalize(request.Query),
		limit:                 limit,
		typeFilter:            typeFilter,
		folderPrefix:          folderPrefix,
		folderPrefixLower:     folderPrefixLower,
		folderPrefixWithSepLC: strings.ToLower(folderPrefixWithSep),
		extension:             normalizeExtension(request.Extension),
	}
}

func cleanSearchPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}

	return filepath.Clean(path)
}

func normalizeExtension(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	if !strings.HasPrefix(value, ".") {
		value = "." + value
	}
	return value
}

func (search compiledSearch) matches(entry Entry) bool {
	if !search.matchesType(entry) {
		return false
	}
	if !search.matchesFolder(entry.Path) {
		return false
	}
	if !search.matchesExtension(entry) {
		return false
	}
	return true
}

func (search compiledSearch) matchesType(entry Entry) bool {
	switch search.typeFilter {
	case "files":
		return !entry.IsDir
	case "folders":
		return entry.IsDir
	default:
		return true
	}
}

func (search compiledSearch) matchesFolder(path string) bool {
	if search.folderPrefix == "" {
		return true
	}

	lowerPath := strings.ToLower(filepath.Clean(path))
	if lowerPath == search.folderPrefixLower {
		return true
	}

	return strings.HasPrefix(lowerPath, search.folderPrefixWithSepLC)
}

func (search compiledSearch) matchesExtension(entry Entry) bool {
	if search.extension == "" {
		return true
	}
	if entry.IsDir {
		return false
	}

	return strings.EqualFold(entry.Ext, search.extension)
}

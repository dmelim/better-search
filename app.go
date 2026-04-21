package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"better-search/internal/indexer"
	"better-search/internal/platform"
)

type App struct {
	ctx context.Context
	idx *indexer.Manager
}

func NewApp() *App {
	return &App{
		idx: indexer.NewManager(platform.DefaultRoots()),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.idx.Start()
}

func (a *App) Status() indexer.Status {
	return a.idx.Status()
}

func (a *App) Search(request indexer.SearchRequest) []indexer.Entry {
	return a.idx.Search(request)
}

func (a *App) Rescan() {
	a.idx.Rescan()
}

func (a *App) OpenPath(path string) error {
	clean := cleanPath(path)
	if clean == "." || clean == "" {
		return fmt.Errorf("missing path")
	}
	return platform.OpenPath(clean)
}

func (a *App) RevealPath(path string) error {
	clean := cleanPath(path)
	if clean == "." || clean == "" {
		return fmt.Errorf("missing path")
	}
	return platform.RevealPath(clean)
}

func (a *App) OpenInVSCode(path string) error {
	clean := cleanPath(path)
	if clean == "." || clean == "" {
		return fmt.Errorf("missing path")
	}
	return platform.OpenInVSCode(clean)
}

func (a *App) ImagePreview(path string) string {
	clean := cleanPath(path)
	if clean == "." || clean == "" {
		return ""
	}

	mimeType := imageMimeType(filepath.Ext(clean))
	if mimeType == "" {
		return ""
	}

	info, err := os.Stat(clean)
	if err != nil || info.IsDir() || info.Size() > 8*1024*1024 {
		return ""
	}

	data, err := os.ReadFile(clean)
	if err != nil {
		return ""
	}

	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)
}

func cleanPath(path string) string {
	return filepath.Clean(strings.TrimSpace(path))
}

func imageMimeType(ext string) string {
	switch strings.ToLower(ext) {
	case ".apng":
		return "image/apng"
	case ".avif":
		return "image/avif"
	case ".bmp":
		return "image/bmp"
	case ".gif":
		return "image/gif"
	case ".ico":
		return "image/x-icon"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".svg":
		return "image/svg+xml"
	case ".webp":
		return "image/webp"
	default:
		return ""
	}
}

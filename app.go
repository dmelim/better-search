package main

import (
	"context"
	"fmt"
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

func cleanPath(path string) string {
	return filepath.Clean(strings.TrimSpace(path))
}

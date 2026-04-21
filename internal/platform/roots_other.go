//go:build !windows

package platform

import (
	"os"
	"path/filepath"
)

func DefaultRoots() []string {
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		return []string{home}
	}

	return []string{filepath.Clean(string(filepath.Separator))}
}

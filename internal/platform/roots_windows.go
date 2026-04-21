//go:build windows

package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"unsafe"
)

const driveTypeFixed = 3

var (
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	procGetLogicalDrives = kernel32.NewProc("GetLogicalDrives")
	procGetDriveTypeW    = kernel32.NewProc("GetDriveTypeW")
)

func DefaultRoots() []string {
	roots := make([]string, 0, 8)
	roots = append(roots, preferredRoots()...)
	roots = append(roots, fixedDriveRoots()...)
	roots = dedupeRoots(roots)
	if len(roots) > 0 {
		return roots
	}

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return []string{`C:\`}
	}

	return []string{filepath.VolumeName(home) + `\`}
}

func fixedDriveRoots() []string {
	mask, _, err := procGetLogicalDrives.Call()
	if mask == 0 {
		return nil
	}
	if err != syscall.Errno(0) && err != nil {
		return nil
	}

	roots := make([]string, 0, 8)
	for i := 0; i < 26; i++ {
		if mask&(1<<uint(i)) == 0 {
			continue
		}

		root := fmt.Sprintf("%c:\\", 'A'+i)
		ptr, convErr := syscall.UTF16PtrFromString(root)
		if convErr != nil {
			continue
		}

		driveType, _, _ := procGetDriveTypeW.Call(uintptr(unsafe.Pointer(ptr)))
		if driveType == driveTypeFixed {
			roots = append(roots, root)
		}
	}

	sort.Strings(roots)
	return roots
}

func preferredRoots() []string {
	roots := make([]string, 0, 8)

	if cwd, err := os.Getwd(); err == nil && cwd != "" {
		roots = append(roots, parentChain(cwd, 3)...)
	}

	if home, err := os.UserHomeDir(); err == nil && home != "" {
		roots = append(roots,
			filepath.Join(home, "Desktop"),
			filepath.Join(home, "Documents"),
			filepath.Join(home, "Downloads"),
			home,
		)
	}

	return existingDirs(roots)
}

func parentChain(path string, limit int) []string {
	path = filepath.Clean(path)
	if path == "." || path == "" {
		return nil
	}

	chain := make([]string, 0, limit+1)
	current := path
	for i := 0; i < limit; i++ {
		chain = append(chain, current)
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	return chain
}

func existingDirs(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			continue
		}
		out = append(out, filepath.Clean(path))
	}
	return out
}

func dedupeRoots(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	out := make([]string, 0, len(paths))

	for _, path := range paths {
		clean := filepath.Clean(path)
		key := strings.ToLower(clean)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, clean)
	}

	return out
}

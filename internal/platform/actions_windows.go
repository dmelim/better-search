//go:build windows

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func OpenPath(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return exec.Command("explorer.exe", path).Start()
	}

	return exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", path).Start()
}

func RevealPath(path string) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}

	clean := filepath.Clean(path)
	return exec.Command("explorer.exe", fmt.Sprintf("/select,%s", clean)).Start()
}

func OpenInVSCode(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return exec.Command("code", "--reuse-window", path).Start()
	}

	return exec.Command("code", "--reuse-window", "--goto", path).Start()
}

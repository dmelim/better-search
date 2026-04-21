//go:build !windows

package platform

import "fmt"

func OpenPath(path string) error {
	return fmt.Errorf("open not implemented for this platform: %s", path)
}

func RevealPath(path string) error {
	return fmt.Errorf("reveal not implemented for this platform: %s", path)
}

func OpenInVSCode(path string) error {
	return fmt.Errorf("VS Code open not implemented for this platform: %s", path)
}

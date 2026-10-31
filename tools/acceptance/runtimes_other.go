//go:build !windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// shortPath is the path itself: only Windows starts a shim through cmd.exe.
func shortPath(path string) string { return path }

// linkProgram links target at link: the kernel starts a symbolic link as
// the program it names.
func linkProgram(target, link string) error {
	if err := os.Symlink(target, link); err != nil {
		return fmt.Errorf("link %s: %w", filepath.Base(link), err)
	}
	return nil
}

package main

import (
	"path/filepath"
	"runtime"
	"testing"
)

// m2UnreadableParent answers a directory path whose children no stat can
// answer as absent: a path through a file, which Unix refuses as not a
// directory while Windows answers it as absent, and on Windows a name with
// a character the file system refuses.
func m2UnreadableParent(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		return filepath.Join(t.TempDir(), "name<refused")
	}
	parent := filepath.Join(t.TempDir(), "file")
	m2Write(t, parent, "occupied\n")
	return parent
}

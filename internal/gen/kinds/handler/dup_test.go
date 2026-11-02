package handler_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/dup"
)

func readTree(t *testing.T, root string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		rel, err := filepath.Rel(root, path)
		files[filepath.ToSlash(rel)] = data
		if err != nil {
			return fmt.Errorf("relative path of %s: %w", path, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestDuplicationSeesACopiedStatusMappingAndNotTheSharedCall(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		root   string
		copied bool
	}{
		"red: each operation copies the status mapping": {root: filepath.Join("testdata", "dup", "red"), copied: true},
		"near: each operation calls statusOf":           {root: filepath.Join("testdata", "dup", "near")},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			report, err := dup.Measure(readTree(t, tc.root), nil, dup.Default())
			if err != nil {
				t.Fatal(err)
			}
			if copied := len(report.Blocks) > 0 && report.Percent() >= 1.5; copied != tc.copied {
				t.Errorf("duplication %.2f percent in %d blocks, want copied %v", report.Percent(), len(report.Blocks), tc.copied)
			}
		})
	}
}

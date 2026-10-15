package golaw_test

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"testing"

	"github.com/cristianargotti/golaw"
)

func TestEverySourceAssetIsEmbedded(t *testing.T) {
	t.Parallel()
	source := os.DirFS(".")
	for _, root := range []string{"agents", "templates"} {
		err := fs.WalkDir(source, root, func(name string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return fmt.Errorf("walk source: %w", walkErr)
			}
			if entry.IsDir() {
				return nil
			}
			return compareAsset(t, source, name)
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

// compareAsset checks one source asset against its embedded bytes.
func compareAsset(t *testing.T, source fs.FS, name string) error {
	t.Helper()
	want, err := fs.ReadFile(source, name)
	if err != nil {
		return fmt.Errorf("read source: %w", err)
	}
	got, err := golaw.FS.ReadFile(name)
	if err != nil {
		t.Errorf("source asset missing from embed: %s: %v", name, err)
		return nil
	}
	if !bytes.Equal(got, want) {
		t.Errorf("embedded bytes differ: %s", name)
	}
	return nil
}

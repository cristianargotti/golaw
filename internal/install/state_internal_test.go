package install

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestProjectIdentity(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	canonical, identity, err := ProjectIdentity(root)
	if err != nil || identity != Digest([]byte(canonical)) {
		t.Fatalf("identity: %q %q %v", canonical, identity, err)
	}
	_, repeated, err := ProjectIdentity(filepath.Join(root, "."))
	if err != nil || repeated != identity {
		t.Fatalf("repeat: %q %v", repeated, err)
	}
	if _, _, missingErr := ProjectIdentity(filepath.Join(root, "absent")); missingErr == nil {
		t.Fatal("missing root accepted")
	}
	file := filepath.Join(root, "file")
	must(t, os.WriteFile(file, nil, 0o600))
	if _, _, fileErr := ProjectIdentity(file); !errors.Is(fileErr, ErrPath) {
		t.Fatalf("a file accepted as the repository root: %v", fileErr)
	}
	if runtime.GOOS == "windows" {
		return
	}
	link := filepath.Join(t.TempDir(), "alias")
	if linkErr := os.Symlink(root, link); linkErr != nil {
		t.Fatal(linkErr)
	}
	_, aliased, err := ProjectIdentity(link)
	if err != nil || aliased != identity {
		t.Fatalf("alias: %q %v", aliased, err)
	}
}

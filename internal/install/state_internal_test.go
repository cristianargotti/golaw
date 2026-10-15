package install

import (
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

package setup_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/runner"
)

func (p *setupProbe) hooksCommand(t *testing.T, action string) runner.Output {
	t.Helper()
	if action == "uninstall" && p.failUndo {
		return runner.Output{Code: 1, Stderr: []byte("uninstall failed\n")}
	}
	for index, hook := range []string{"pre-commit", "commit-msg", "pre-push"} {
		path := filepath.Join(p.hooks, hook)
		switch action {
		case "install":
			installHook(t, path)
			if p.failHook && index == 0 {
				return runner.Output{Code: 1, Stderr: []byte("interrupted install\n")}
			}
		case "uninstall":
			uninstallHook(t, path)
		default:
			t.Fatalf("unexpected hook action: %s", action)
		}
	}
	p.checksumFile(t, action)
	return runner.Output{}
}

// installHook writes the owned hook, moving a foreign one aside as
// lefthook does.
func installHook(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	if err == nil && string(data) != fakeHook {
		if renameErr := os.Rename(path, path+".old"); renameErr != nil {
			t.Fatal(renameErr)
		}
	}
	setupWrite(t, path, fakeHook)
}

// uninstallHook removes the owned hook and puts back the foreign one it
// moved aside.
func uninstallHook(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	if err := os.Rename(path+".old", path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
}

// checksumFile writes the checksum lefthook install leaves and removes it
// on uninstall.
func (p *setupProbe) checksumFile(t *testing.T, action string) {
	t.Helper()
	switch action {
	case "install":
		setupWrite(t, p.checksum, "checksum\n")
	case "uninstall":
		if err := os.Remove(p.checksum); err != nil && !errors.Is(err, fs.ErrNotExist) {
			t.Fatal(err)
		}
	}
}

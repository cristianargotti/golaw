package profile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A placement that fails after moving the old bundle aside puts it back;
// nothing moved aside is no failure, and a copy that cannot go back is
// named, because the person then has to install the version again.
func TestRestoreLeavesAnAbsentCopyAndNamesAFailedOne(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dst := filepath.Join(root, "fixture")
	if err := restore(filepath.Join(root, "absent"), dst); err != nil {
		t.Fatalf("nothing moved aside = %v", err)
	}
	aside := filepath.Join(root, "old")
	for _, dir := range []string{aside, filepath.Join(dst, "held")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := restore(aside, dst); !errors.Is(err, ErrBundle) || !strings.Contains(err.Error(), "restore "+dst) {
		t.Fatalf("a copy that cannot go back = %v", err)
	}
}

// Every file and directory of a bundle copy names the path it could not
// read or write, so a failed install points at the cause.
func TestCopyFileAndMakeDirNameWhatFails(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := copyFile(filepath.Join(root, "absent.yml"), filepath.Join(root, "copy.yml")); !errors.Is(err, ErrBundle) || !strings.Contains(err.Error(), "read ") {
		t.Fatalf("an absent file = %v", err)
	}
	file := filepath.Join(root, RulesFile)
	if err := os.WriteFile(file, []byte("golaw_profile: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := copyFile(file, filepath.Join(root, "absent", RulesFile)); !errors.Is(err, ErrBundle) || !strings.Contains(err.Error(), "write ") {
		t.Fatalf("a target under an absent directory = %v", err)
	}
	if err := makeDir(filepath.Join(file, "docs")); !errors.Is(err, ErrBundle) || !strings.Contains(err.Error(), "create ") {
		t.Fatalf("a directory under a file = %v", err)
	}
}

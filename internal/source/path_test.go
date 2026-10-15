package source_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/cristianargotti/golaw/internal/source"
)

func TestCleanRel(t *testing.T) {
	t.Parallel()
	good := map[string]string{
		"a.go":             "a.go",
		"./dir/a.go":       "dir/a.go",
		`dir\sub\a.go`:     "dir/sub/a.go",
		"dir//a.go":        "dir/a.go",
		"dir/../b/../c.go": "c.go",
	}
	for in, want := range good {
		if got, err := source.CleanRel(in); err != nil || got != want {
			t.Errorf("CleanRel(%q) = %q, %v, want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", ".", "..", "../x", "a/../../x", "/abs", `\abs`} {
		if _, err := source.CleanRel(in); !errors.Is(err, source.ErrEscapedRoot) {
			t.Errorf("CleanRel(%q) = %v, want ErrEscapedRoot", in, err)
		}
	}
}

func TestOSPath(t *testing.T) {
	t.Parallel()
	root := filepath.Join("r", "o")
	if got := source.OSPath(root, "a/b.go"); got != filepath.Join(root, "a", "b.go") {
		t.Errorf("OSPath = %q", got)
	}
}

func TestRel(t *testing.T) {
	t.Parallel()
	root := initRepo(t)
	write(t, root, "internal/a.go", "package a\n")
	cases := map[string]string{
		filepath.Join(root, "internal", "a.go"):  "internal/a.go",
		"internal/a.go":                          "internal/a.go",
		filepath.Join("internal", "new", "b.go"): "internal/new/b.go",
		filepath.Join(root, "x", "..", "c.go"):   "c.go",
	}
	for in, want := range cases {
		if got, err := source.Rel(root, root, in); err != nil || got != want {
			t.Errorf("Rel(%q) = %q, %v, want %q", in, got, err, want)
		}
	}
	outside := t.TempDir()
	for _, in := range []string{"", root, filepath.Join(root, ".."), outside, filepath.Join(outside, "x.go"), "../x.go"} {
		if _, err := source.Rel(root, root, in); !errors.Is(err, source.ErrEscapedRoot) {
			t.Errorf("Rel(%q) = %v, want ErrEscapedRoot", in, err)
		}
	}
	if got, err := source.Rel(root, filepath.Join(root, "internal"), "a.go"); err != nil || got != "internal/a.go" {
		t.Errorf("Rel relative to a subdirectory = %q, %v", got, err)
	}
}

func TestRelResolvesLinks(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links need a privilege on Windows")
	}
	root := initRepo(t)
	write(t, root, "docs/a.md", "a\n")
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(root, filepath.Join(outside, "alias")); err != nil {
		t.Fatal(err)
	}
	if _, err := source.Rel(root, root, filepath.Join(root, "escape", "x.go")); !errors.Is(err, source.ErrUnsafeLink) {
		t.Errorf("through a link that leads out: %v, want ErrUnsafeLink", err)
	}
	if got, err := source.Rel(root, root, filepath.Join(outside, "alias", "docs", "a.md")); err != nil || got != "docs/a.md" {
		t.Errorf("through a link that leads in: %q, %v", got, err)
	}
	if got, err := source.Rel(root, root, filepath.Join(outside, "alias", "new", "b.md")); err != nil || got != "new/b.md" {
		t.Errorf("a path to create through a link that leads in: %q, %v", got, err)
	}
}

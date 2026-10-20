package bootstrap_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestReadOwnershipKeepsTheFirstExistingLocation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"top", map[string]string{
			".github/CODEOWNERS": "* @top\n", "CODEOWNERS": "* @root\n", "docs/CODEOWNERS": "* @docs\n",
		}, "* @top\n"},
		{"root", map[string]string{"CODEOWNERS": "* @root\n", "docs/CODEOWNERS": "* @docs\n"}, "* @root\n"},
		{"docs", map[string]string{"docs/CODEOWNERS": "* @docs\n"}, "* @docs\n"},
		{"empty top", map[string]string{".github/CODEOWNERS": "", "CODEOWNERS": "* @root\n"}, ""},
		{"empty root", map[string]string{"CODEOWNERS": "", "docs/CODEOWNERS": "* @docs\n"}, ""},
		{"absent", nil, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeOwnershipFiles(t, root, tc.files)
			got, err := bootstrap.ReadOwnership(root)
			if err != nil || string(got) != tc.want {
				t.Fatalf("ownership = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestReadOwnershipRefusesUnsafeLocationsWithoutFallingThrough(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		want error
	}{
		{"file link", bootstrap.ErrSymlink},
		{"parent link", bootstrap.ErrSymlink},
		{"directory", install.ErrPath},
		{"parent file", install.ErrPath},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeOwnershipFiles(t, root, map[string]string{"CODEOWNERS": "* @fallback\n"})
			makeUnsafeOwnership(t, root, tc.name)
			got, err := bootstrap.ReadOwnership(root)
			if !errors.Is(err, tc.want) || got != nil {
				t.Fatalf("unsafe ownership fell through: %q, %v", got, err)
			}
		})
	}
}

func makeUnsafeOwnership(t *testing.T, root, kind string) {
	t.Helper()
	parent := filepath.Join(root, ".github")
	switch kind {
	case "parent link":
		if err := os.Symlink(t.TempDir(), parent); err != nil {
			t.Fatal(err)
		}
	case "parent file":
		writeOwnershipFiles(t, root, map[string]string{".github": "ordinary file\n"})
	case "directory":
		if err := os.MkdirAll(filepath.Join(parent, "CODEOWNERS"), 0o750); err != nil {
			t.Fatal(err)
		}
	case "file link":
		if err := os.Mkdir(parent, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(t.TempDir(), "missing"), filepath.Join(parent, "CODEOWNERS")); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatalf("unknown unsafe ownership %q", kind)
	}
}

func TestReadOwnershipEnforcesThePolicyReadBound(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	at := strings.Repeat("#", model.MaxPolicyBytes)
	writeOwnershipFiles(t, root, map[string]string{".github/CODEOWNERS": at, "CODEOWNERS": "* @fallback\n"})
	got, err := bootstrap.ReadOwnership(root)
	if err != nil || string(got) != at {
		t.Fatalf("exact bound = %d bytes, %v", len(got), err)
	}
	writeOwnershipFiles(t, root, map[string]string{".github/CODEOWNERS": at + "#"})
	got, err = bootstrap.ReadOwnership(root)
	if !errors.Is(err, bootstrap.ErrTooLarge) || got != nil {
		t.Fatalf("oversized ownership fell through: %d bytes, %v", len(got), err)
	}
}

func TestReadOwnershipValidatesItsRoot(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	file := filepath.Join(parent, "file")
	writeOwnershipFiles(t, parent, map[string]string{"file": "ordinary file\n"})
	for _, tc := range []struct {
		path string
		want error
	}{
		{file, install.ErrPath},
		{filepath.Join(parent, "missing"), fs.ErrNotExist},
	} {
		got, err := bootstrap.ReadOwnership(tc.path)
		if !errors.Is(err, tc.want) || got != nil {
			t.Fatalf("invalid root accepted: %q, %v", got, err)
		}
	}
}

func writeOwnershipFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, data := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

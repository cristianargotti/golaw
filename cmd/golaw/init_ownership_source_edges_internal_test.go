package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestInitOwnershipEmptyLocationWins(t *testing.T) {
	t.Parallel()
	for _, top := range []string{".github/CODEOWNERS", "CODEOWNERS"} {
		t.Run(top, func(t *testing.T) {
			t.Parallel()
			files := map[string]string{"CODEOWNERS": "* @root\n", "docs/CODEOWNERS": "* @docs\n"}
			files[top] = ""
			for _, committed := range []bool{false, true} {
				requireOwnershipLocation(t, files, "", committed)
			}
		})
	}
}

func TestInitOwnershipCheckoutRefusesUnsafeFirstLocation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		want error
	}{
		{"file link", bootstrap.ErrSymlink},
		{"parent link", bootstrap.ErrSymlink},
		{"directory", install.ErrPath},
		{"large", bootstrap.ErrTooLarge},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := gittest.New(t)
			repo.Write("CODEOWNERS", "* @fallback\n")
			unsafeOwnershipLocation(t, repo, tc.name)
			ta := initBindApp(t)
			data, err := ta.bind().initOwnership(t.Context(), repo.Root)
			if !errors.Is(err, tc.want) || data != nil {
				t.Fatalf("unsafe location fell through: %q, %v", data, err)
			}
		})
	}
}

func unsafeOwnershipLocation(t *testing.T, repo *gittest.Repo, kind string) {
	t.Helper()
	parent := filepath.Join(repo.Root, ".github")
	if kind == "parent link" {
		if err := os.Symlink(t.TempDir(), parent); err != nil {
			t.Fatal(err)
		}
		return
	}
	if err := os.MkdirAll(parent, 0o750); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(parent, "CODEOWNERS")
	switch kind {
	case "file link":
		if err := os.Symlink(filepath.Join(t.TempDir(), "missing"), target); err != nil {
			t.Fatal(err)
		}
	case "directory":
		if err := os.Mkdir(target, 0o750); err != nil {
			t.Fatal(err)
		}
	case "large":
		repo.Write(".github/CODEOWNERS", strings.Repeat("#", model.MaxPolicyBytes+1))
	default:
		t.Fatalf("unknown unsafe location %q", kind)
	}
}

func TestInitOwnershipCheckoutAcceptsTheExactReadBound(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	want := strings.Repeat("#", model.MaxPolicyBytes)
	repo.Write(".github/CODEOWNERS", want)
	repo.Write("CODEOWNERS", "* @fallback\n")
	ta := initBindApp(t)
	data, err := ta.bind().initOwnership(t.Context(), repo.Root)
	if err != nil || string(data) != want {
		t.Fatalf("bounded ownership: %d bytes, %v", len(data), err)
	}
}

func TestInitOwnershipCommittedReadErrorsNeverUseCheckout(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"symlink", "large"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			repo := gittest.New(t)
			want := model.ErrUnreadable
			files := map[string]string{"CODEOWNERS": "* @fallback\n"}
			if kind == "large" {
				files[".github/CODEOWNERS"] = strings.Repeat("#", model.MaxReadBytes+1)
				want = model.ErrTooLarge
			} else {
				unsafeOwnershipLocation(t, repo, "file link")
				repo.Git("add", "--", ".github/CODEOWNERS")
			}
			repo.Commit("chore: establish unreadable ownership", files)
			repo.Remove(".github/CODEOWNERS")
			repo.Write(".github/CODEOWNERS", "* @checkout\n")
			ta := initBindApp(t)
			data, err := ta.bind().initOwnership(t.Context(), repo.Root)
			if !errors.Is(err, want) || data != nil {
				t.Fatalf("committed read fell back: %q, %v", data, err)
			}
		})
	}
}

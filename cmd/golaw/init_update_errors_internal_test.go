package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/gittest"
)

func TestInitUpdateRejectsInvalidInputsWithoutWriting(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, want string }{
		{"outside checkout", "not a git repository root"},
		{"policy directory", "read policy to update"},
		{"module directory", "read module input"},
		{"invalid policy", "decode update settings"},
		{"ambiguous layout", "both docs/adr and docs/decisions"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := gittest.New(t)
			repo.Write(bootstrap.PolicyFile, "golaw: 1\nkit: {version: 9.9.8}\n")
			root := invalidUpdateInput(t, repo, tc.name)
			ta := initBindApp(t)
			code := ta.run(t.Context(), []string{"init", "--update", "--apply", root})
			if code != exitFailure || !strings.Contains(ta.stderr.String(), tc.want) || ta.stdout.Len() != 0 {
				t.Fatalf("invalid input = %d, %q, %q", code, ta.stdout.String(), ta.stderr.String())
			}
			requireNoInitTemplates(t, repo.Root)
			dirs, err := ta.bind().dirs()
			if err != nil {
				t.Fatal(err)
			}
			if entries, err := os.ReadDir(dirs.State); err == nil && len(entries) != 0 {
				t.Fatalf("invalid input enrolled: %v", entries)
			}
		})
	}
}

func invalidUpdateInput(t *testing.T, repo *gittest.Repo, name string) string {
	t.Helper()
	switch name {
	case "outside checkout":
		return t.TempDir()
	case "policy directory":
		if err := os.Remove(filepath.Join(repo.Root, bootstrap.PolicyFile)); err != nil {
			t.Fatal(err)
		}
		mkdirUpdateInput(t, repo.Root, bootstrap.PolicyFile)
	case "module directory":
		mkdirUpdateInput(t, repo.Root, "go.mod")
	case "invalid policy":
		repo.Write(bootstrap.PolicyFile, "golaw: 1\nunknown: true\n")
	case "ambiguous layout":
		mkdirUpdateInput(t, repo.Root, "docs/adr")
		mkdirUpdateInput(t, repo.Root, "docs/decisions")
	default:
		t.Fatalf("unknown input %q", name)
	}
	return repo.Root
}

func mkdirUpdateInput(t *testing.T, root, name string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(name)), 0o750); err != nil {
		t.Fatal(err)
	}
}

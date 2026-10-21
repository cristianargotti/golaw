package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
)

func TestInitCIUsesExistingOwnershipAndRemainsUnarmed(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, owners string
		protected    bool
	}{
		{"core", "* @base\n", false},
		{"same owner", "* @base\n/.github/ @base\n", false},
		{"foreign owner", "* @base\n/.github/ @other\n", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			requireInitCI(t, test.owners, test.protected)
		})
	}
}

func requireInitCI(t *testing.T, owners string, protected bool) {
	t.Helper()
	const destination = ".github/workflows/ci.yml"
	repo := gittest.New(t)
	repo.Commit("feat: establish service", map[string]string{
		"go.mod":     "module example.test/service\n\ngo 1.27\n",
		"CODEOWNERS": owners,
	})
	ta := initBindApp(t)
	path := filepath.Join(repo.Root, filepath.FromSlash(destination))
	if code := ta.run(t.Context(), []string{"init", repo.Root}); code != exitClean {
		t.Fatalf("preview exit=%d: %s", code, ta.stderr.String())
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("preview wrote CI: %v", err)
	}
	ta.stdout.Reset()
	if code := ta.run(t.Context(), []string{"init", "--apply", repo.Root}); code != exitClean {
		t.Fatalf("apply exit=%d: %s", code, ta.stderr.String())
	}
	data, err := os.ReadFile(path)
	if protected {
		if !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("foreign destination was written: %v", err)
		}
	} else if err != nil || !strings.Contains(string(data), "make ci") {
		t.Fatalf("generated CI=%q: %v", data, err)
	}
	times := managedTimes(t, repo.Root)
	_, managed := times[destination]
	if managed == protected {
		t.Fatalf("managed=%t, protected=%t", managed, protected)
	}
	requireUnarmed(t, repo.GitDir())
	ta.stdout.Reset()
	if code := ta.run(t.Context(), []string{"init", "--apply", repo.Root}); code != exitClean {
		t.Fatalf("repeat exit=%d: %s", code, ta.stderr.String())
	}
	requireSilentApply(t, ta.stdout.String(), repo.Root, times)
	requireUnarmed(t, repo.GitDir())
}

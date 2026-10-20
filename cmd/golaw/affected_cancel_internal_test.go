package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/runnertest"
)

//nolint:paralleltest // The command retains git plumbing's process environment.
func TestAffectedCommandRemovesItsTreeWhenTheContextEnds(t *testing.T) {
	repo := gittest.New(t)
	repo.Commit("chore: seed", map[string]string{
		"go.mod":        "module example.test/cancel\n\ngo 1.24\n",
		"leaf/value.go": "package leaf\nconst Value = 1\n",
	})
	repo.Stage(map[string]string{
		"leaf/value.go": "package leaf\nconst Value = 2\n",
	})
	ta := ladderApp(t)
	b := ta.bind()
	dirs, err := b.dirs()
	if err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(dirs.State, "runner", "trees")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	sawTree := false
	b.environ = func() []string {
		entries, readErr := os.ReadDir(parent)
		if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
			t.Fatal(readErr)
		}
		if len(entries) > 0 {
			sawTree = true
			cancel()
		}
		return append(gittest.Isolated(), "GOWORK=off", "GOPROXY=off")
	}
	ta.bind = func() binding { return b }
	code := ta.run(ctx, []string{"affected", repo.Root})
	if !sawTree {
		t.Fatal("cancellation did not observe a materialized tree")
	}
	if code != exitFailure ||
		!strings.Contains(ta.stderr.String(), context.Canceled.Error()) {
		t.Fatalf("exit = %d, stderr = %q", code, ta.stderr.String())
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatalf("resources remain = %v, %v", entries, err)
	}
}

// The fake Go executable replaces the process search path.
func TestAffectedCommandDoesNoWorkForAnUnchangedIndex(t *testing.T) {
	repo := gittest.New(t)
	repo.Commit("chore: seed", map[string]string{"value.go": "package value\n"})
	bin := t.TempDir()
	runnertest.Install(t, bin, "go", runnertest.Behavior{Code: 9})
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	ta := ladderApp(t)
	b := ta.bind()
	dirs, err := b.dirs()
	if err != nil {
		t.Fatal(err)
	}
	b.environ = func() []string {
		t.Fatal("an unchanged index requested an execution environment")
		return nil
	}
	ta.bind = func() binding { return b }
	if code := ta.run(t.Context(), []string{"affected", repo.Root}); code != exitClean {
		t.Fatalf("exit = %d, stderr = %q", code, ta.stderr.String())
	}
	if ta.stdout.Len() != 0 || ta.stderr.Len() != 0 {
		t.Fatalf("stdout = %q, stderr = %q", ta.stdout.String(), ta.stderr.String())
	}
	if _, err := os.Stat(filepath.Join(dirs.State, "runner", "trees")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("empty index created tree state: %v", err)
	}
	if calls := runnertest.Calls(t, bin, "go"); len(calls) != 0 {
		t.Fatalf("empty index started Go: %v", calls)
	}
}

func TestAffectedCommandKeepsBothSidesOfARename(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	repo.Commit("chore: seed", map[string]string{
		"go.mod":       "module example.test/moved\n\ngo 1.24\n",
		"old/value.go": "package leaf\nconst Value = 1\n",
		"use/value.go": "package use\nimport leaf \"example.test/moved/old\"\nconst Value = leaf.Value\n",
	})
	repo.StageRename("old/value.go", "new/value.go")
	ta := ladderApp(t)
	b := ta.bind()
	b.environ = func() []string {
		return append(gittest.Isolated(), "GOWORK=off", "GOPROXY=off")
	}
	ta.bind = func() binding { return b }
	if code := ta.run(t.Context(), []string{"affected", repo.Root}); code != exitClean {
		t.Fatalf("exit = %d, stderr = %q", code, ta.stderr.String())
	}
	if got := ta.stdout.String(); got != "./new\n./use\n" {
		t.Fatalf("renamed package and former importers = %q", got)
	}
}

func affectedFailureRepo(t *testing.T) *gittest.Repo {
	t.Helper()
	repo := gittest.New(t)
	repo.Commit("chore: seed", map[string]string{
		"go.mod":   "module example.test/failure\n\ngo 1.24\n",
		"value.go": "package failure\nconst Value = 1\n",
	})
	repo.Stage(map[string]string{"value.go": "package failure\nconst Value = 2\n"})
	return repo
}

func TestAffectedCommandReportsPreparationFailures(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"repository", "index", "state", "sweep", "materialize"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			repo := affectedFailureRepo(t)
			ta := ladderApp(t)
			b := ta.bind()
			root, want := prepareAffectedFailure(t, kind, repo, &b)
			ta.bind = func() binding { return b }
			if code := ta.run(t.Context(), []string{"affected", root}); code != exitFailure {
				t.Fatalf("exit = %d, stderr = %q", code, ta.stderr.String())
			}
			if !strings.Contains(ta.stderr.String(), want) || ta.stdout.Len() != 0 {
				t.Fatalf("stdout = %q, stderr = %q, want %q", ta.stdout.String(), ta.stderr.String(), want)
			}
		})
	}
}

func prepareAffectedFailure(t *testing.T, kind string, repo *gittest.Repo, b *binding) (string, string) {
	t.Helper()
	switch kind {
	case "repository":
		return t.TempDir(), "discover"
	case "index":
		if err := os.WriteFile(filepath.Join(repo.GitDir(), "index"), []byte("invalid"), 0o600); err != nil {
			t.Fatal(err)
		}
		return repo.Root, "read staged changes"
	case "state":
		b.env = func(string) string { return "" }
		return repo.Root, "state directories"
	case "sweep":
		dirs, err := b.dirs()
		if err != nil {
			t.Fatal(err)
		}
		parent := filepath.Join(dirs.State, "runner")
		if err := os.MkdirAll(parent, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(parent, "trees"), []byte("occupied"), 0o600); err != nil {
			t.Fatal(err)
		}
		return repo.Root, "sweep staged trees"
	default:
		b.environ = func() []string {
			return append(gittest.Isolated(), "GIT_INDEX_FILE="+repo.Root)
		}
		return repo.Root, "materialize staged tree"
	}
}

// The fake Go executable replaces the process search path.
func TestAffectedCommandRemovesItsTreeWhenGoFails(t *testing.T) {
	repo := affectedFailureRepo(t)
	bin := t.TempDir()
	runnertest.Install(t, bin, "go", runnertest.Behavior{
		Code: 7, Stderr: "package listing failed\n",
	})
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	ta := ladderApp(t)
	b := ta.bind()
	_, err := b.affected(t.Context(), repo.Root)
	if !errors.Is(err, runner.ErrPackages) || !strings.Contains(err.Error(), "package listing failed") {
		t.Fatalf("package failure = %v", err)
	}
	if calls := runnertest.Calls(t, bin, "go"); len(calls) != 1 {
		t.Fatalf("Go calls = %v", calls)
	}
	dirs, err := b.dirs()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(dirs.State, "runner", "trees"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("resources remain = %v, %v", entries, err)
	}
}

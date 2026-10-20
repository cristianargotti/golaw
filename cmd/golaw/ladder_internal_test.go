package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/runner"
)

func ladderApp(t *testing.T) testApp {
	t.Helper()
	ta := bindApp(t)
	b := ta.bind()
	b.environ = gittest.Isolated
	ta.bind = func() binding { return b }
	return ta
}

// Git's alternate index and cwd belong to this test.
func TestAffectedCommandUsesTheAlternateIndex(t *testing.T) {
	repo := gittest.New(t)
	repo.Commit("chore: seed", map[string]string{
		"go.mod":        "module example.test/helper\n\ngo 1.24\n",
		"leaf/value.go": "package leaf\nconst Value = 1\n",
		"use/value.go": `package use
import "example.test/helper/leaf"
const Value = leaf.Value
`,
	})
	index := repo.AlternateIndex()
	repo.Write("leaf/value.go", "package leaf\nconst Value = 2\n")
	repo.GitEnv([]string{"GIT_INDEX_FILE=" + index}, "add", "--", "leaf/value.go")
	repo.Write("use/value.go", "package use\nconst Value = 1\n")
	t.Chdir(repo.Root)
	t.Setenv("GIT_INDEX_FILE", index)
	ta := ladderApp(t)
	b := ta.bind()
	b.environ = func() []string {
		return append(gittest.Isolated(),
			"GIT_DIR="+repo.GitDir(), "GIT_INDEX_FILE="+index,
			"GOWORK=off", "GOPROXY=off",
		)
	}
	ta.bind = func() binding { return b }
	if got := ta.run(t.Context(), []string{"affected", repo.Root}); got != exitClean {
		t.Fatalf("exit = %d, stderr = %q", got, ta.stderr.String())
	}
	if got := ta.stdout.String(); got != "./leaf\n./use\n" {
		t.Fatalf("patterns = %q", got)
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

//nolint:paralleltest // The helper has no path argument and uses cwd.
func TestPushRangeCommandReadsEveryStdinLine(t *testing.T) {
	repo := gittest.New(t)
	base := repo.Commit("chore: seed", map[string]string{"base.txt": "base\n"})
	tip := repo.Branch("topic", map[string]string{"topic.txt": "topic\n"})
	repo.Git("update-ref", "refs/remotes/origin/main", base)
	t.Chdir(repo.Root)
	ta := ladderApp(t)
	ta.stdin = strings.NewReader(
		"refs/heads/topic " + tip + " refs/heads/topic " + runner.ZeroSHA + "\n" +
			"(delete) " + runner.ZeroSHA + " refs/heads/gone " + base + "\n",
	)
	if got := ta.run(t.Context(), []string{"push-range", "origin"}); got != exitClean {
		t.Fatalf("exit = %d, stderr = %q", got, ta.stderr.String())
	}
	if got := ta.stdout.String(); got != base+" "+tip+"\n" {
		t.Fatalf("ranges = %q", got)
	}
}

//nolint:paralleltest // The helper has no path argument and uses cwd.
func TestPushRangeCommandPrintsAnAbsentBaseWithoutShiftingFields(t *testing.T) {
	repo := gittest.New(t)
	tip := repo.Commit("chore: seed", map[string]string{
		"base.txt": "base\n",
	})
	t.Chdir(repo.Root)
	ta := ladderApp(t)
	zero := strings.Repeat("0", len(tip))
	ta.stdin = strings.NewReader(
		"refs/heads/main " + tip + " refs/heads/main " + zero + "\n",
	)
	if got := ta.run(t.Context(), []string{
		"push-range", "https://example.test/repository.git",
	}); got != exitClean {
		t.Fatalf("exit = %d, stderr = %q", got, ta.stderr.String())
	}
	if got := ta.stdout.String(); got != zero+" "+tip+"\n" {
		t.Fatalf("ranges = %q", got)
	}
}

func TestLadderHelpersRejectMalformedArguments(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{"affected", "one", "two"},
		{"push-range"},
		{"push-range", "one", "two"},
		{"lock", "name"},
		{"lock", "../outside", "--", "git"},
		{"lock", "name", "git"},
	} {
		ta := ladderApp(t)
		if got := ta.run(t.Context(), args); got != exitFailure ||
			ta.stderr.Len() == 0 {
			t.Fatalf("args=%v exit=%d stderr=%q", args, got, ta.stderr.String())
		}
	}
}

//nolint:paralleltest // Push-range discovers the repository through the process cwd.
func TestPushRangeCommandReportsInputAndRangeFailures(t *testing.T) {
	repo := gittest.New(t)
	repo.Commit("chore: seed", map[string]string{"value.txt": "value\n"})
	for _, tc := range []struct {
		name, dir, input, want string
	}{
		{"discovery", t.TempDir(), "", "discover"},
		{"malformed", repo.Root, "invalid\n", "read push updates"},
		{
			"missing-tip", repo.Root,
			"refs/heads/main " + strings.Repeat("f", 40) + " refs/heads/main " + runner.ZeroSHA + "\n",
			"resolve push ranges",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(tc.dir)
			ta := ladderApp(t)
			ta.stdin = strings.NewReader(tc.input)
			if code := ta.run(t.Context(), []string{"push-range", "origin"}); code != exitFailure {
				t.Fatalf("exit = %d, stderr = %q", code, ta.stderr.String())
			}
			if !strings.Contains(ta.stderr.String(), tc.want) || ta.stdout.Len() != 0 {
				t.Fatalf("stdout = %q, stderr = %q, want %q", ta.stdout.String(), ta.stderr.String(), tc.want)
			}
		})
	}
}

//nolint:paralleltest // Push-range discovers the repository through the process cwd.
func TestPushRangeCommandPropagatesOutputFailure(t *testing.T) {
	repo := gittest.New(t)
	tip := repo.Commit("chore: seed", map[string]string{"value.txt": "value\n"})
	t.Chdir(repo.Root)
	ta := ladderApp(t)
	ta.app.stdout = coverageWriteFailure{}
	ta.stdin = strings.NewReader("refs/heads/main " + tip + " refs/heads/main " + runner.ZeroSHA + "\n")
	if code := ta.run(t.Context(), []string{"push-range", "origin"}); code != exitFailure {
		t.Fatalf("exit = %d, stderr = %q", code, ta.stderr.String())
	}
	if !strings.Contains(ta.stderr.String(), errBoom.Error()) {
		t.Fatalf("output error = %q", ta.stderr.String())
	}
}

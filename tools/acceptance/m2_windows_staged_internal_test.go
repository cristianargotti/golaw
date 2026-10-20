package main

import (
	"context"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// TestM2WindowsPreCommitReadsAStagedGoChange holds the Windows row to a Go
// change in the index when its installed pre-commit runs: with nothing
// staged lefthook skips the rung and the rung answers its gitleaks,
// golangci-lint and go vet steps without starting them, so the recorder
// PATH would prove nothing of them.
func TestM2WindowsPreCommitReadsAStagedGoChange(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t).m2Scope("windows-staged")
	s.m2OS = goosWindows
	script := m2NativeScript(t, s, m2WindowsOnly, "answer")
	var staged []string
	s.execute = func(ctx context.Context, c command) result {
		if m2InstalledHook(s, c) && filepath.Base(c.args[0]) == "pre-commit" {
			staged = append(staged,
				m2IndexAtHook(ctx, t, s, c, "diff", "--cached", "--numstat"),
				m2IndexAtHook(ctx, t, s, c, "show", ":"+m2SamplePath))
		}
		return script.Execute(ctx, c)
	}
	if got := s.nativeM2(t.Context(), m2WindowsOnly); got.status != statusPass || !got.required {
		t.Fatalf("Windows row = %+v", got)
	}
	if len(staged) != 2 || staged[0] != "2\t0\t"+m2SamplePath+"\n" {
		t.Fatalf("installed pre-commit saw the staged change %q, want two added lines of %s once", staged, m2SamplePath)
	}
	m2RequireFormattedComment(t, staged[1])
}

func TestM2WindowsSampleStagingRejectsUnavailableState(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"write", "add"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			s, repo := m2PreparedNative(t)
			s.execute = m2SampleStageFault(t, s, repo, mode)
			if err := s.m2StageSampleChange(t.Context(), repo); err == nil {
				t.Fatalf("accepted %s failure", mode)
			}
		})
	}
}

// m2SampleStageFault breaks the staging of the sample change: write turns
// the sample into a directory, add fails the git add of the sample.
func m2SampleStageFault(t *testing.T, s *suite, repo, mode string) executor {
	t.Helper()
	if mode == "write" {
		path := filepath.Join(repo, m2SamplePath)
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(path, dirMode); err != nil {
			t.Fatal(err)
		}
	}
	fixture := s.execute
	return func(ctx context.Context, c command) result {
		if mode == "add" && c.program == s.git && slices.Equal(c.args, []string{gitAdd, "--", m2SamplePath}) {
			return result{err: ErrAcceptance}
		}
		return fixture(ctx, c)
	}
}

// m2RequireFormattedComment holds the staged sample to the committed body
// and one comment line that ends a sentence, formatted as gofmt writes it:
// the lint step refuses a formatter or godot issue on a staged line, which
// would fail the Windows row for its fixture instead of its PATH.
func m2RequireFormattedComment(t *testing.T, staged string) {
	t.Helper()
	added, found := strings.CutPrefix(staged, m2SampleBody)
	if !found || !regexp.MustCompile(`^\n// [^\n]+\.\n$`).MatchString(added) {
		t.Fatalf("staged sample = %q, want the committed body and one comment line", staged)
	}
	formatted, err := format.Source([]byte(staged))
	if err != nil || string(formatted) != staged {
		t.Fatalf("staged sample is not formatted: %q, %v", formatted, err)
	}
}

// m2IndexAtHook asks the real git of the suite about the index of the
// repository an installed hook runs in, with the environment of the hook.
func m2IndexAtHook(ctx context.Context, t *testing.T, s *suite, c command, args ...string) string {
	t.Helper()
	got := runCommand(ctx, command{program: s.git, args: args, dir: c.dir, env: c.env})
	if err := success(got); err != nil {
		t.Fatalf("git %q at the installed hook: %v", args, err)
	}
	return got.stdout
}

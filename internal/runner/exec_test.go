package runner_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/runner"
)

func git(r *gittest.Repo, stdin string, args ...string) runner.Command {
	c := runner.Command{Name: "git", Args: args, Dir: r.Root, Env: gittest.Isolated()}
	if stdin != "" {
		c.Stdin = strings.NewReader(stdin)
	}
	return c
}

func TestRunAnswersTheOutputAndTheStatus(t *testing.T) {
	t.Parallel()
	r := gittest.New(t)
	out, err := runner.Run(t.Context(), git(r, "", "version"))
	if err != nil || out.Code != 0 || !strings.HasPrefix(string(out.Stdout), "git version ") {
		t.Errorf("git version: %+v, %v", out, err)
	}
	out, err = runner.Run(t.Context(), git(r, "", "rev-parse", "--verify", "-q", "no-such-ref"))
	if err != nil || out.Code != 1 || len(out.Stdout) != 0 {
		t.Errorf("a status other than zero is an output, not an error: %+v, %v", out, err)
	}
	out, err = runner.Run(t.Context(), git(r, "", "no-such-command"))
	if err != nil || out.Code == 0 || !strings.Contains(string(out.Stderr), "no-such-command") {
		t.Errorf("stderr is kept apart: %+v, %v", out, err)
	}
}

func TestRunHandsTheCommandItsInput(t *testing.T) {
	t.Parallel()
	r := gittest.New(t)
	r.Write("golaw.txt", "golaw\n")
	want := r.Git("hash-object", "golaw.txt")
	out, err := runner.Run(t.Context(), git(r, "golaw\n", "hash-object", "--stdin"))
	if err != nil || strings.TrimSpace(string(out.Stdout)) != want {
		t.Errorf("hash of stdin = %q, %v, want %s", out.Stdout, err, want)
	}
	out, err = runner.Run(t.Context(), git(r, "", "hash-object", "--stdin"))
	if err != nil || strings.TrimSpace(string(out.Stdout)) != "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391" {
		t.Errorf("no input is the empty blob: %q, %v", out.Stdout, err)
	}
}

func TestRunRefusesAProgramThatCannotStart(t *testing.T) {
	t.Parallel()
	_, err := runner.Run(t.Context(), runner.Command{Name: "golaw-no-such-program", Dir: t.TempDir()})
	if !errors.Is(err, runner.ErrStart) || !strings.Contains(err.Error(), "golaw-no-such-program") {
		t.Errorf("err = %v, want ErrStart naming the program", err)
	}
}

func TestRunStopsWhenTheContextEnds(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := runner.Run(ctx, git(gittest.New(t), "", "version"))
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want the context's error", err)
	}
}

func TestRunKeepsAtMostTheBoundOfEachStream(t *testing.T) {
	t.Parallel()
	r := gittest.New(t)
	r.Stage(map[string]string{"big.txt": strings.Repeat("x", runner.MaxOutputBytes+1024)})
	out, err := runner.Run(t.Context(), git(r, "", "show", ":big.txt"))
	if err != nil || out.Code != 0 || len(out.Stdout) != runner.MaxOutputBytes {
		t.Errorf("kept %d bytes, code %d, %v; want %d", len(out.Stdout), out.Code, err, runner.MaxOutputBytes)
	}
}

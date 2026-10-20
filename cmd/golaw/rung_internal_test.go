package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks"
	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/hook/agents"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/setup"
)

// rungApp is the test app over the production binding and catalog, run
// from the root of the repository, as git runs a hook, in a repository
// golaw setup armed.
func rungApp(t *testing.T, root string) testApp {
	t.Helper()
	ta := bindApp(t)
	ta.registry = func() (*engine.Registry, error) { return checks.Registry() }
	armed(t, ta, root)
	t.Chdir(root)
	return ta
}

// armed writes the receipt golaw setup leaves for the repository at root in
// the state directory of the app's binding: the hooks directory it armed.
func armed(t *testing.T, ta testApp, root string) {
	t.Helper()
	dirs, err := ta.bind().dirs()
	if err != nil {
		t.Fatal(err)
	}
	canonical, identity, err := install.ProjectIdentity(root)
	if err != nil {
		t.Fatal(err)
	}
	receipt := setup.Receipt{
		Schema: setup.ReceiptSchema, Identity: identity, Root: canonical, Version: testVersion,
		HooksPath: filepath.Join(canonical, ".git", "hooks"), ArmedAt: now(),
	}
	if err := setup.WriteReceipt(setup.ReceiptPath(dirs.State, identity), receipt); err != nil {
		t.Fatal(err)
	}
}

func TestHookRefusesTheArgumentsNoRungTakes(t *testing.T) {
	t.Parallel()
	agent := agents.IDs()[0]
	cases := map[string][]string{
		"commit-msg without its file": {"hook", "commit-msg"},
		"pre-commit with a file":      {"hook", "pre-commit", "x"},
		"pre-push without its URL":    {"hook", "pre-push", "origin"},
		"an agent with an argument":   {"hook", agent, "extra"},
		"more than the longest rung":  {"hook", "pre-push", "origin", "url", "extra"},
		"neither an agent nor a rung": {"hook", "post-merge"},
		"no agent and no rung":        {"hook"},
	}
	for name, args := range cases {
		ta := newTestApp(t)
		if got := ta.run(t.Context(), args); got != exitFailure || ta.stdout.Len() != 0 || !strings.HasPrefix(ta.stderr.String(), "golaw: ") {
			t.Errorf("%s: exit %d, stdout %q, stderr %q", name, got, ta.stdout.String(), ta.stderr.String())
		}
	}
}

//nolint:paralleltest // the rung runs in the directory of the repository, which t.Chdir sets
func TestHookRunsTheCommitMsgRungOfTheRepository(t *testing.T) {
	root := repository(t, true)
	message := filepath.Join(root, ".git", "COMMIT_EDITMSG")
	if err := os.WriteFile(message, []byte("Update the reader\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ta := rungApp(t, root)
	if got := ta.run(t.Context(), []string{"hook", "commit-msg", ".git/COMMIT_EDITMSG"}); got != exitFindings ||
		!strings.Contains(ta.stdout.String(), "deny commit.conventional the subject \"Update the reader\"") || ta.stderr.Len() != 0 {
		t.Fatalf("exit %d, stdout %q, stderr %q", got, ta.stdout.String(), ta.stderr.String())
	}
	if err := os.WriteFile(message, []byte("feat: add the reader\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	clean := rungApp(t, root)
	if got := clean.run(t.Context(), []string{"hook", "commit-msg", ".git/COMMIT_EDITMSG"}); got != exitClean || !strings.HasPrefix(clean.stdout.String(), "verdict clean: 0 findings") {
		t.Fatalf("exit %d, stdout %q, stderr %q", got, clean.stdout.String(), clean.stderr.String())
	}
}

//nolint:paralleltest // the rung runs in the directory of the repository, which t.Chdir sets
func TestHookRefusesAPinOfAnotherVersionAndRunsNothingSwitchedOff(t *testing.T) {
	root := repository(t, false)
	if err := os.WriteFile(filepath.Join(root, "golaw.yml"), []byte("golaw: 1\nkit: {version: 0.0.1}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "build: pin an older kit")
	ta := rungApp(t, root)
	if got := ta.run(t.Context(), []string{"hook", "pre-commit"}); got != exitFailure ||
		!strings.Contains(ta.stdout.String(), "pins kit version 0.0.1 and this binary is 9.9.9: install the pinned version or run init --update") {
		t.Fatalf("exit %d, stdout %q, stderr %q", got, ta.stdout.String(), ta.stderr.String())
	}
	if err := os.WriteFile(filepath.Join(root, "golaw.yml"), []byte("golaw: 1\nkit: {version: 0.0.1}\nladder: {pre_commit: false, commit_msg: false, pre_push: false}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"hook", "pre-commit"}, {"hook", "commit-msg", "absent"}, {"hook", "pre-push", "origin", "url"}, {"hook", "pre-push"}} {
		off := rungApp(t, root)
		off.stdin = strings.NewReader("malformed")
		if got := off.run(t.Context(), args); got != exitClean || off.stdout.Len() != 0 || off.stderr.Len() != 0 {
			t.Errorf("%v switched off: exit %d, stdout %q, stderr %q", args, got, off.stdout.String(), off.stderr.String())
		}
	}
}

// A lefthook dispatcher another tool installed reads the lefthook.yml init
// writes, so init alone would arm the rungs through it; the rung answers
// nothing until golaw setup armed the hooks directory.
//
//nolint:paralleltest // the rung runs in the directory of the repository, which t.Chdir sets
func TestHookRunsNoRungSetupDidNotArm(t *testing.T) {
	root := repository(t, true)
	dispatch := filepath.Join(root, ".git", "hooks", "commit-msg")
	if err := os.WriteFile(dispatch, []byte("#!/bin/sh\nexec lefthook run commit-msg \"$@\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "COMMIT_EDITMSG"), []byte("Update the reader\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ta := initBindApp(t)
	ta.registry = func() (*engine.Registry, error) { return checks.Registry() }
	t.Chdir(root)
	if got := ta.run(t.Context(), []string{"init", "--apply", root}); got != exitClean {
		t.Fatalf("init --apply: exit %d, stderr %q", got, ta.stderr.String())
	}
	ta.stdout.Reset()
	if got := ta.run(t.Context(), []string{"hook", "commit-msg", ".git/COMMIT_EDITMSG"}); got != exitClean || ta.stdout.Len() != 0 || ta.stderr.Len() != 0 {
		t.Fatalf("a rung golaw setup did not arm: exit %d, stdout %q, stderr %q", got, ta.stdout.String(), ta.stderr.String())
	}
	armed(t, ta, root)
	if got := ta.run(t.Context(), []string{"hook", "commit-msg", ".git/COMMIT_EDITMSG"}); got != exitFindings || !strings.Contains(ta.stdout.String(), "deny commit.conventional") {
		t.Fatalf("the rung golaw setup armed: exit %d, stdout %q", got, ta.stdout.String())
	}
}

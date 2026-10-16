package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	caseExisting = "existing-repository"
	// commitStep is the step agent install names while the reviewed policy
	// of a repository with commits is not committed yet.
	commitStep = "commit the reviewed golaw.yml"
)

// existingRow replays the adoption of a repository that already has a
// commit, the sequence docs/setup.md gives: agent install refuses until the
// policy init --apply wrote is committed and names that step, installs once
// a commit holds it, and scan is clean. The row keeps a home of its own, so
// its user scope installation never touches the receipt the other rows
// read.
func (s *suite) existingRow(ctx context.Context) row {
	repo := filepath.Join(s.base, dirRepos, caseExisting)
	env := isolatedEnvironment(os.Environ(), filepath.Join(s.base, "existing home"), filepath.Dir(s.binary))
	golaw := func(args ...string) result {
		return s.execute(ctx, command{program: s.binary, args: args, dir: repo, env: env})
	}
	err := s.startModule(ctx, repo)
	if err == nil {
		err = success(golaw(commandInit, flagApply))
	}
	if err == nil {
		err = refusedUntilCommitted(golaw(installArgs()...))
	}
	if err == nil {
		err = s.commitPolicy(ctx, repo, false)
	}
	if err == nil {
		err = success(golaw(installArgs()...))
	}
	if err == nil {
		err = success(golaw(commandScan, "."))
	}
	return measured(categoryInstallation, caseExisting, err)
}

// startModule creates a repository whose first commit holds the module
// file only: a project whose history began before it adopted the kit.
func (s *suite) startModule(ctx context.Context, repo string) error {
	module := []byte("module example.invalid/existing\n\ngo 1.27\n")
	if err := writeFile(filepath.Join(repo, moduleFile), module, fileMode); err != nil {
		return err
	}
	return s.gitSteps(ctx, repo, []string{commandInit, "-q"}, []string{gitAdd, "--", moduleFile}, commitArgs("build: start the module"))
}

// refusedUntilCommitted holds the first agent install of the row to the
// refusal that names the commit step, never a silent install or a refusal
// that leaves the person at a missing policy.
func refusedUntilCommitted(got result) error {
	if got.err != nil {
		return fmt.Errorf("execute process: %w", got.err)
	}
	if got.code != 2 || !strings.Contains(got.stderr, commitStep) {
		return fmt.Errorf("agent install before the commit exited %d with %q: %w",
			got.code, strings.TrimSpace(got.stderr), ErrAcceptance)
	}
	return nil
}

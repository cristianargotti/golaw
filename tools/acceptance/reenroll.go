package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	caseReenroll = "reenroll-tightening"
	// enrollmentStep starts the line agent install prints when it moves the
	// enrollment record to the policy in force.
	enrollmentStep = "enrollment: policy "
	// reenrollStep is what scan names while a reviewed change waits for it.
	reenrollStep   = "run agent install"
	tightenedLimit = 200
)

// reenrollRow replays the repair of a reviewed policy change (A3): after
// the adoption of a repository with commits, a commit tightens golaw.yml
// to 200 lines a file; scan refuses the mismatch and names agent install,
// agent install names the re-enrollment and enrolls the policy in force,
// scan is clean again, and the next writes are judged under the tightened
// policy: 200 lines are allowed and 201 denied. The row keeps a home of
// its own, so its installation never touches the receipt the other rows
// read.
func (s *suite) reenrollRow(ctx context.Context) row {
	repo := filepath.Join(s.base, dirRepos, caseReenroll)
	env := isolatedEnvironment(os.Environ(), filepath.Join(s.base, "reenroll home"), filepath.Dir(s.binary))
	golaw := func(args ...string) result {
		return s.execute(ctx, command{program: s.binary, args: args, dir: repo, env: env})
	}
	steps := []func() error{
		func() error { return s.startModule(ctx, repo) },
		func() error { return success(golaw(commandInit, flagApply)) },
		func() error { return s.commitPolicy(ctx, repo, false) },
		func() error { return success(golaw(installArgs()...)) },
		func() error { return s.tightenPolicy(ctx, repo) },
		func() error { return answered(golaw(commandScan, "."), 2, reenrollStep) },
		func() error { return answered(golaw(installArgs()...), 0, enrollmentStep) },
		func() error { return success(golaw(commandScan, ".")) },
		func() error {
			f := writeFixture(caseReenroll+"-200", pathSize, goLines(tightenedLimit), wantAllow)
			return s.judgeIn(ctx, env, repo, f)
		},
		func() error {
			f := writeFixture(caseReenroll+"-201", pathSize, goLines(tightenedLimit+1), wantDeny)
			return s.judgeIn(ctx, env, repo, f)
		},
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return measured(categoryInstallation, caseReenroll, err)
		}
	}
	return measured(categoryInstallation, caseReenroll, nil)
}

// tightenPolicy commits the reviewed tightening of the row: 200 lines a
// file instead of the 300 init rendered.
func (s *suite) tightenPolicy(ctx context.Context, repo string) error {
	path := filepath.Join(repo, policyFile)
	body, err := readFile(path)
	if err != nil {
		return err
	}
	tightened := bytes.Replace(body, []byte("file_lines: 300"), []byte(fmt.Sprintf("file_lines: %d", tightenedLimit)), 1)
	if bytes.Equal(tightened, body) {
		return fmt.Errorf("generated policy lacks file_lines: %w", ErrAcceptance)
	}
	if err := writeFile(path, tightened, fileMode); err != nil {
		return err
	}
	return s.gitSteps(ctx, repo, []string{gitAdd, "--", policyFile}, commitArgs("build: tighten the policy"))
}

// answered holds a command to its exit code and to the step its output
// names, so a refusal for another cause never passes.
func answered(got result, code int, text string) error {
	if got.err != nil {
		return success(got)
	}
	if got.code != code || !strings.Contains(got.stdout+got.stderr, text) {
		return fmt.Errorf("exit %d with %q, want exit %d naming %q: %w",
			got.code, strings.TrimSpace(got.stdout+got.stderr), code, text, ErrAcceptance)
	}
	return nil
}

// judgeIn sends a fixture through the installed invocation from the home
// of a row of its own.
func (s *suite) judgeIn(ctx context.Context, env []string, repo string, f fixture) error {
	body, err := payload(s.root, repo, f)
	if err != nil {
		return err
	}
	return expect(s.invokeIn(ctx, env, repo, f.event, f.tool, body), f.want)
}

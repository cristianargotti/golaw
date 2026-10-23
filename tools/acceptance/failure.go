package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	faultStartup      = "startup"
	faultCrash        = "crash"
	faultEmpty        = "empty"
	faultTimeout      = "timeout"
	hostTimeout       = 10 * time.Second
	slowTimeout       = 9 * time.Second
	categoryTransport = "failure-transport"
	// categoryProtocolLimit holds the empty output fault: exit 0 with no
	// output is the allow of the host protocol, so the row proves that the
	// fault was injected and observed, never a denial.
	categoryProtocolLimit = "protocol-limit"
	categoryRecovery      = "recovery"
	caseDeadline          = "internal-deadline"
	caseRecovery          = "recovery-control"
)

func (s *suite) failureRows(ctx context.Context) []row {
	source := filepath.Join(s.root, dirTools, dirAcceptance, "testdata", "fault.go.txt")
	fault, err := s.buildFault(ctx, source)
	if err != nil {
		return []row{measured(categoryTransport, "build-fault", err)}
	}
	repo, err := s.repository(ctx, "faults", true)
	if err != nil {
		return []row{measured(categoryTransport, "prepare-fault", err)}
	}
	f := writeFixture("faults", pathNote, textSafe, wantAllow)
	input, err := payload(s.root, repo, f)
	if err != nil {
		return []row{measured(categoryTransport, "fault-payload", err)}
	}
	original, err := readFile(s.binary)
	if err != nil {
		return []row{measured(categoryTransport, "read-binary", err)}
	}
	replacement, err := readFile(fault)
	if err != nil {
		return []row{measured(categoryTransport, "read-fault", err)}
	}
	var rows []row
	for _, mode := range []string{faultStartup, faultCrash, faultEmpty, faultTimeout} {
		rows = append(rows, s.faultRow(ctx, mode, repo, input, original, replacement))
	}
	return append(rows, s.slowRow(ctx, fault, repo, input), s.recoveryRow(ctx, repo, original))
}

// recoveryRow replays the rollback of section 9 from a missing binary: the
// validated binary put back and agent install run again restore the
// allowed write through the installed invocation.
func (s *suite) recoveryRow(ctx context.Context, repo string, original []byte) row {
	input, err := payload(s.root, repo, writeFixture(caseRecovery, pathNote, textSafe, wantAllow))
	if err != nil {
		return measured(categoryRecovery, caseRecovery, err)
	}
	if err := os.Remove(s.binary); err != nil {
		return measured(categoryRecovery, caseRecovery, fmt.Errorf("remove the binary: %w", err))
	}
	broken := s.invoke(ctx, repo, eventPre, toolWrite, input)
	if err := writeFile(s.binary, original, execMode); err != nil {
		return measured(categoryRecovery, caseRecovery, err)
	}
	if broken.err == nil {
		return measured(categoryRecovery, caseRecovery,
			fmt.Errorf("the installed hook answered without its binary: %w", ErrAcceptance))
	}
	reinstall := command{
		program: s.binary, args: installArgs(),
		dir: filepath.Join(s.base, dirRepos, repoInstallation), env: s.env,
	}
	if err := success(s.execute(ctx, reinstall)); err != nil {
		return measured(categoryRecovery, caseRecovery, err)
	}
	got := s.invoke(ctx, repo, eventPre, toolWrite, input)
	return measured(categoryRecovery, caseRecovery, expect(got, wantAllow))
}

func (s *suite) buildFault(ctx context.Context, source string) (string, error) {
	return s.buildProgram(ctx, source, "fault")
}

func (s *suite) faultRow(ctx context.Context, mode, repo string, input, original, replacement []byte) row {
	err := s.replaceWithFault(mode, replacement)
	got := result{err: err}
	if err == nil {
		child, cancel := context.WithTimeout(ctx, hostTimeout)
		got = s.invoke(child, repo, eventPre, toolWrite, input)
		cancel()
	}
	restoreErr := writeFile(s.binary, original, execMode)
	removeErr := os.Remove(s.binary + ".mode")
	if errors.Is(removeErr, fs.ErrNotExist) {
		removeErr = nil
	}
	err = errors.Join(err, restoreErr, removeErr, faultResult(mode, got))
	if mode == faultEmpty {
		item := measured(categoryProtocolLimit, mode, err)
		if item.status == statusPass {
			item.detail = "fault observed; exit 0 with no output is an allow to the host, so the row records the fault and proves no denial"
		}
		return item
	}
	item := measured(categoryTransport, mode, err)
	if item.status == statusPass {
		item.detail = "fault observed; installed onFailure is block; live dispatch not inferred"
	}
	return item
}

func (s *suite) replaceWithFault(mode string, replacement []byte) error {
	if err := os.Remove(s.binary); err != nil {
		return fmt.Errorf("remove temporary binary: %w", err)
	}
	if mode == faultStartup {
		return nil
	}
	if err := writeFile(s.binary, replacement, execMode); err != nil {
		return err
	}
	return writeFile(s.binary+".mode", []byte(mode), fileMode)
}

func (s *suite) slowRow(ctx context.Context, fault, repo string, input []byte) row {
	dir := filepath.Join(s.base, "slowgit")
	git := filepath.Join(dir, executable("git"))
	body, err := readFile(fault)
	if err != nil {
		return measured(wantFailure, caseDeadline, err)
	}
	if err := writeFile(git, body, execMode); err != nil {
		return measured(wantFailure, caseDeadline, err)
	}
	if err := writeFile(git+".mode", []byte("slowgit"), fileMode); err != nil {
		return measured(wantFailure, caseDeadline, err)
	}
	if err := writeFile(git+".real", []byte(s.git), fileMode); err != nil {
		return measured(wantFailure, caseDeadline, err)
	}
	original := s.env
	s.env = prependPath(s.env, dir)
	child, cancel := context.WithTimeout(ctx, slowTimeout)
	got := s.invoke(child, repo, eventPre, toolWrite, input)
	cancel()
	s.env = original
	if expect(got, wantError) == nil && strings.Contains(got.stderr, "deadline") {
		return measured(wantFailure, caseDeadline, nil)
	}
	return measured(wantFailure, caseDeadline,
		fmt.Errorf("real binary did not answer before host timeout: %w", ErrAcceptance))
}

func faultResult(mode string, got result) error {
	failed := got.err != nil
	observed := false
	switch mode {
	case faultStartup, faultTimeout:
		observed = failed
	case faultCrash:
		observed = !failed && got.code == 2 && strings.TrimSpace(got.stderr) != ""
	case faultEmpty:
		observed = !failed && got.code == 0 && got.stdout == "" && got.stderr == ""
	}
	if observed {
		return nil
	}
	return fmt.Errorf("fault injection did not produce %s: %w", mode, ErrAcceptance)
}

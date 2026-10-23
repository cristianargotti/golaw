package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/policy"
)

// The repository and the commands of a profile row.
const (
	m5Module           = "module example.test/fixture\n\ngo 1.27\n"
	m5Capabilities     = "go,image"
	m5CapabilitiesFlag = "--capabilities"
	m5ProfileCommand   = "profile"
	m5InstallSub       = "install"
	m5ProfileFlag      = "--profile"
	m5DoctorCommand    = "doctor"
	m5InstallCommand   = "golaw profile install " + m5Canonical + "@" + m5Version
	m5DoctorProfile    = "doctor.profile"
	m5AdoptMessage     = "feat: adopt the golaw policy"
)

// m5Golaw runs the candidate in dir with the scope's environment.
func (s *suite) m5Golaw(ctx context.Context, dir string, args ...string) result {
	return s.execute(ctx, command{program: s.binary, args: args, dir: dir, env: s.env})
}

// m5Profile runs one subcommand of the profile lifecycle, which needs no
// repository, from the scope's base.
func (s *suite) m5Profile(ctx context.Context, sub, arg string) result {
	if err := os.MkdirAll(s.base, dirMode); err != nil {
		return result{err: fmt.Errorf("create the base of the case: %w", err)}
	}
	return s.m5Golaw(ctx, s.base, m5ProfileCommand, sub, arg)
}

// m5Dirs are the state and config directories the candidate resolves in
// the scope's home, through the same function it calls.
func (s *suite) m5Dirs() (policy.Dirs, error) {
	dirs, err := policy.DefaultDirs(runtime.GOOS, func(key string) string { return environmentValue(s.env, key) })
	if err != nil {
		return policy.Dirs{}, fmt.Errorf("resolve the golaw directories of the case: %w", err)
	}
	return dirs, nil
}

// m5Install installs the bundle at tag from source into the scope's home.
func (s *suite) m5Install(ctx context.Context, source, tag string) (result, error) {
	got := s.m5Profile(ctx, m5InstallSub, source+"@"+tag)
	if err := success(got); err != nil {
		return got, fmt.Errorf("install the fixture bundle at %s: %w", tag, err)
	}
	return got, nil
}

// m5Current is the stamp of the current bundle of the fixture.
func (s *suite) m5Current() (policy.Stamp, error) {
	dirs, err := s.m5Dirs()
	if err != nil {
		return policy.Stamp{}, err
	}
	stamp, err := policy.LoadStamp(policy.StampPath(dirs.Config, m5Name))
	if err != nil {
		return policy.Stamp{}, fmt.Errorf("read the current stamp: %w", err)
	}
	return stamp, nil
}

// m5Repository is a committed module repository adopted with the go and
// image capabilities, under the fixture profile when profiled.
func (s *suite) m5Repository(ctx context.Context, name string, profiled bool) (string, error) {
	repo, err := s.repository(ctx, name, false)
	if err != nil {
		return "", err
	}
	if stepErr := writeFile(filepath.Join(repo, moduleFile), []byte(m5Module), fileMode); stepErr != nil {
		return "", stepErr
	}
	if stepErr := s.gitSteps(ctx, repo, []string{commandInit, m2Quiet}); stepErr != nil {
		return "", stepErr
	}
	args := []string{commandInit, m5CapabilitiesFlag, m5Capabilities, flagApply}
	if profiled {
		args = append(args, m5ProfileFlag, m5Name)
	}
	if stepErr := success(s.m5Golaw(ctx, repo, args...)); stepErr != nil {
		return "", fmt.Errorf("adopt the repository %s: %w", name, stepErr)
	}
	return repo, s.gitSteps(ctx, repo, []string{gitAdd, "."}, commitArgs(m5AdoptMessage))
}

// m5DoctorLine finds a line of doctor --json, which prints one document
// per agent, so the documents are decoded in turn.
func m5DoctorLine(stdout, id string) (doctor.Line, error) {
	decoder := json.NewDecoder(strings.NewReader(stdout))
	for {
		var report doctor.Report
		err := decoder.Decode(&report)
		if errors.Is(err, io.EOF) {
			return doctor.Line{}, fmt.Errorf("doctor --json carries no %s line: %w", id, ErrAcceptance)
		}
		if err != nil {
			return doctor.Line{}, fmt.Errorf("decode doctor --json: %w", err)
		}
		for _, line := range report.Lines {
			if line.ID == id {
				return line, nil
			}
		}
	}
}

// m5Doctor is the profile line of doctor --json over a repository.
func (s *suite) m5Doctor(ctx context.Context, repo string) (doctor.Line, error) {
	got := s.m5Golaw(ctx, repo, m5DoctorCommand, m2JSONFlag)
	if err := success(got); err != nil {
		return doctor.Line{}, err
	}
	return m5DoctorLine(got.stdout, m5DoctorProfile)
}

// m5ExpectProfile requires the doctor profile line of a repository to
// carry code and, in its detail, fragment.
func (s *suite) m5ExpectProfile(ctx context.Context, repo, code, fragment string) error {
	line, err := s.m5Doctor(ctx, repo)
	if err != nil {
		return err
	}
	if line.Key != m5Name || line.Code != code || !strings.Contains(line.Detail, fragment) {
		return fmt.Errorf("doctor.profile is %s %s %q, want %s naming %q: %w", line.Key, line.Code, line.Detail, code, fragment, ErrAcceptance)
	}
	return nil
}

// m5OneLine judges a governed operation without its bundle (A19, D8): it
// fails, and exactly one line of what it printed names the install
// command, so the person reads one error with its remedy.
func m5OneLine(got result) error {
	if got.err != nil {
		return fmt.Errorf("execute the governed operation: %w", got.err)
	}
	if got.code == 0 {
		return fmt.Errorf("the operation passed without the bundle: %w", ErrAcceptance)
	}
	count := 0
	for line := range strings.SplitSeq(strings.ReplaceAll(got.stdout+"\n"+got.stderr, "\r\n", "\n"), "\n") {
		if !strings.Contains(line, "golaw profile install") {
			continue
		}
		count++
		if !strings.Contains(line, m5InstallCommand) {
			return fmt.Errorf("the refusal names %q, want %q: %w", strings.TrimSpace(line), m5InstallCommand, ErrAcceptance)
		}
	}
	if count != 1 {
		return fmt.Errorf("%d lines name the install command, want one: %w", count, ErrAcceptance)
	}
	return nil
}

package main

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/rung"
)

// The reviewed debt set of A24: its record, the inherited finding, the
// init flag, and the words of the two refusals.
const (
	m5DebtRecord = "docs/adr/0002-inherited-debt.md"
	m5DebtNotes  = "notes.md"
	m5SeedFlag   = "--seed-debt"
	m5SeededOnce = "the debt set is seeded once"
	m5Weakens    = "weakens exceptions"
	m5DashCheck  = "text.dashes"
	m5StagedFlag = "--staged"
	m5ShiftDays  = 30
	m5DayHours   = 24
	m5EmDash     = rune(0x2014)
)

// m5DashNotes is a note with one typographic dash, a finding with a path,
// the one kind D13 seeds; the rune is built, never written.
func m5DashNotes() []byte {
	return []byte("# Notes\n\nThe service " + string(m5EmDash) + " in short.\n")
}

// m5CommitDay is the day of the last commit: the tool reads no clock.
func (s *suite) m5CommitDay(ctx context.Context, repo string) (string, error) {
	got := s.execute(ctx, command{program: s.git, args: []string{"log", "-1", "--format=%cs"}, dir: repo, env: s.env})
	if err := success(got); err != nil {
		return "", err
	}
	return strings.TrimSpace(got.stdout), nil
}

// m5SeededRepository is a repository with one inherited finding, adopted
// with --seed-debt and committed; it answers the seeded entry.
func (s *suite) m5SeededRepository(ctx context.Context, name string) (string, policy.ExceptionRule, error) {
	repo, err := s.repository(ctx, name, false)
	if err != nil {
		return "", policy.ExceptionRule{}, err
	}
	for path, body := range map[string][]byte{moduleFile: []byte(m5Module), m5DebtNotes: m5DashNotes()} {
		if stepErr := writeFile(filepath.Join(repo, path), body, fileMode); stepErr != nil {
			return "", policy.ExceptionRule{}, stepErr
		}
	}
	if stepErr := s.gitSteps(ctx, repo, []string{commandInit, m2Quiet}, []string{gitAdd, "."}, commitArgs("chore: render the service")); stepErr != nil {
		return "", policy.ExceptionRule{}, stepErr
	}
	day, err := s.m5CommitDay(ctx, repo)
	if err != nil {
		return "", policy.ExceptionRule{}, err
	}
	if stepErr := success(s.m5Golaw(ctx, repo, commandInit, m5SeedFlag, flagApply)); stepErr != nil {
		return "", policy.ExceptionRule{}, stepErr
	}
	entry, err := m5ReadSeeded(repo, day)
	if err != nil {
		return "", policy.ExceptionRule{}, err
	}
	return repo, entry, s.gitSteps(ctx, repo, []string{gitAdd, "."}, commitArgs(m5AdoptMessage))
}

// m5ReadSeeded judges the seeded policy and the record that lists it.
func m5ReadSeeded(repo, day string) (policy.ExceptionRule, error) {
	data, err := readFile(filepath.Join(repo, policyFile))
	if err != nil {
		return policy.ExceptionRule{}, err
	}
	file, err := policy.Decode(data)
	if err != nil {
		return policy.ExceptionRule{}, fmt.Errorf("decode the seeded policy: %w", err)
	}
	entry, err := m5JudgeSeeded(file, day)
	if err != nil {
		return policy.ExceptionRule{}, err
	}
	record, err := readFile(filepath.Join(repo, filepath.FromSlash(m5DebtRecord)))
	if err != nil {
		return policy.ExceptionRule{}, err
	}
	if !bytes.Contains(record, []byte(entry.Check)) || !bytes.Contains(record, []byte(entry.Path)) {
		return policy.ExceptionRule{}, fmt.Errorf("the debt record does not list %s at %s: %w", entry.Check, entry.Path, ErrAcceptance)
	}
	return entry, nil
}

// m5JudgeSeeded: one exception, the inherited finding at its path with the
// record, until 180 days after the day of the adoption, give or take one.
func m5JudgeSeeded(file policy.File, day string) (policy.ExceptionRule, error) {
	if len(file.Exceptions) != 1 {
		return policy.ExceptionRule{}, fmt.Errorf("%d exceptions seeded, want one: %w", len(file.Exceptions), ErrAcceptance)
	}
	entry := file.Exceptions[0]
	if entry.Check != m5DashCheck || entry.Path != m5DebtNotes || entry.Record != m5DebtRecord {
		return policy.ExceptionRule{}, fmt.Errorf("the seeded entry is %+v: %w", entry, ErrAcceptance)
	}
	start, err := time.Parse(time.DateOnly, day)
	if err != nil {
		return policy.ExceptionRule{}, fmt.Errorf("parse the day of the adoption: %w", err)
	}
	until, err := time.Parse(time.DateOnly, entry.Until)
	if err != nil {
		return policy.ExceptionRule{}, fmt.Errorf("parse the expiry of the entry: %w", err)
	}
	if days := int(until.Sub(start).Hours() / m5DayHours); days < 179 || days > 181 {
		return policy.ExceptionRule{}, fmt.Errorf("the entry expires %d days after the adoption, want 180: %w", days, ErrAcceptance)
	}
	return entry, nil
}

// m5ShiftUntil moves the one occurrence of a day by days.
func m5ShiftUntil(data []byte, until string, days int) ([]byte, error) {
	if bytes.Count(data, []byte(until)) != 1 {
		return nil, fmt.Errorf("golaw.yml names %s %d times, want once: %w", until, bytes.Count(data, []byte(until)), ErrAcceptance)
	}
	day, err := time.Parse(time.DateOnly, until)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", until, err)
	}
	return bytes.Replace(data, []byte(until), []byte(day.AddDate(0, 0, days).Format(time.DateOnly)), 1), nil
}

// m5Arm arms the git rungs of one repository with the pinned tools the
// preparation installed, as a person does with golaw setup.
func (s *suite) m5Arm(ctx context.Context, repo string) ([]string, error) {
	env, err := s.m2PrerequisiteEnvironment(s.env, s.m2.Bin)
	if err != nil {
		return nil, err
	}
	if stepErr := success(s.execute(ctx, command{program: s.binary, args: []string{"setup"}, dir: repo, env: env})); stepErr != nil {
		return nil, fmt.Errorf("arm the rungs of %s: %w", repo, stepErr)
	}
	return env, nil
}

// m5JudgeBroadening: scan --staged refuses the proposed policy that
// broadens an entry, on stderr alone.
func m5JudgeBroadening(got result) error {
	if got.err != nil || got.code != 2 || got.stdout != "" || !strings.Contains(got.stderr, m5Weakens) {
		return fmt.Errorf("the staged broadening answered %d %q %q, want 2 naming %q: %w", got.code, got.stdout, got.stderr, m5Weakens, ErrAcceptance)
	}
	return nil
}

func (s *suite) m5SeededOnce(ctx context.Context) error {
	repo, _, err := s.m5SeededRepository(ctx, "seeded")
	if err != nil {
		return err
	}
	if stepErr := success(s.m5Golaw(ctx, repo, commandScan, ".")); stepErr != nil {
		return fmt.Errorf("the seeded exception does not hold the inherited finding: %w", stepErr)
	}
	before, err := readFile(filepath.Join(repo, policyFile))
	if err != nil {
		return err
	}
	again := s.m5Golaw(ctx, repo, commandInit, m5SeedFlag, flagApply)
	if again.err != nil || again.code == 0 || !strings.Contains(again.stdout+again.stderr, m5SeededOnce) {
		return fmt.Errorf("a second seed answered %d: %w", again.code, ErrAcceptance)
	}
	if stepErr := success(s.m5Golaw(ctx, repo, commandInit, flagApply)); stepErr != nil {
		return stepErr
	}
	after, err := readFile(filepath.Join(repo, policyFile))
	if err != nil {
		return err
	}
	if !bytes.Equal(before, after) {
		return fmt.Errorf("golaw.yml changed after the seed: %w", ErrAcceptance)
	}
	return nil
}

func (s *suite) m5Broadening(ctx context.Context) error {
	repo, entry, err := s.m5SeededRepository(ctx, "broadening")
	if err != nil {
		return err
	}
	env, err := s.m5Arm(ctx, repo)
	if err != nil {
		return err
	}
	if stepErr := s.m5StageUntil(ctx, repo, entry.Until, m5ShiftDays); stepErr != nil {
		return stepErr
	}
	if stepErr := m5JudgeBroadening(s.m5Golaw(ctx, repo, commandScan, m5StagedFlag, ".")); stepErr != nil {
		return stepErr
	}
	refusal := s.execute(ctx, command{program: s.binary, args: []string{commandHook, rung.PreCommit}, dir: repo, env: env})
	if stepErr := m2JudgeNative(refusal, true, m5Weakens, false); stepErr != nil {
		return stepErr
	}
	if stepErr := s.m5StageUntil(ctx, repo, entry.Until, -m5ShiftDays); stepErr != nil {
		return stepErr
	}
	return success(s.m5Golaw(ctx, repo, commandScan, m5StagedFlag, "."))
}

// m5StageUntil stages golaw.yml with the expiry of the entry moved by days
// from the committed one.
func (s *suite) m5StageUntil(ctx context.Context, repo, until string, days int) error {
	committed := s.execute(ctx, command{program: s.git, args: []string{"show", "HEAD:" + policyFile}, dir: repo, env: s.env})
	if err := success(committed); err != nil {
		return err
	}
	shifted, err := m5ShiftUntil([]byte(committed.stdout), until, days)
	if err != nil {
		return err
	}
	if stepErr := writeFile(filepath.Join(repo, policyFile), shifted, fileMode); stepErr != nil {
		return stepErr
	}
	return s.gitSteps(ctx, repo, []string{gitAdd, policyFile})
}

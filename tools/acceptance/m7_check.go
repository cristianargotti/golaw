package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen"
)

// The check rows of M7.13 row 5: the summary of a clean check (contract
// 2.11), and the three files a drift row edits: a golaw regenerable file,
// the generated half of the SQL adapter and the sqlc output of its query.
const (
	m7CheckCategory = "m7-check"
	m7CheckClean    = "golaw gen --check: clean, 0 drift, 0 open holes"
	m7FakePath      = "internal/application/growth/ledgertest/zz_generated_fake.go"
	m7PlansPath     = "internal/adapters/out/postgres/zz_generated_plans.go"
	m7QueriesPath   = "internal/adapters/out/postgres/sqlcgen/plans.sql.go"
)

// m7HandEdit is the line a drift row appends, as a person's edit would.
const m7HandEdit = "// edited by hand\n"

// m7DriftLine is the drift line of contract 2.11 with the fix it names.
func m7DriftLine(path, cause string) string {
	return "golaw gen --check: drift " + path + " " + cause + ": run golaw gen"
}

// m7HoleLine is the end of the hole line of contract 2.11; the path and
// the line of the hole come before it.
func m7HoleLine(id string) string {
	return " " + id + ": fill it and its red test"
}

// m7JudgeCheck requires golaw gen --check to run, exit code and print every
// line of want.
func m7JudgeCheck(got result, code int, want []string) error {
	if got.err != nil {
		return fmt.Errorf("golaw gen --check did not run: %w", got.err)
	}
	if got.code != code {
		return fmt.Errorf("golaw gen --check exit %d, want %d: %s: %w", got.code, code, strings.TrimSpace(got.stdout+"\n"+got.stderr), ErrAcceptance)
	}
	for _, line := range want {
		if !strings.Contains(got.stdout, line) {
			return fmt.Errorf("golaw gen --check lacks %q: %s: %w", line, strings.TrimSpace(got.stdout), ErrAcceptance)
		}
	}
	return nil
}

// m7CheckCases are the rows of golaw gen --check; each runs the pinned
// generators.
func m7CheckCases() []m7Case {
	return []m7Case{
		{name: "clean-after-fill", gen: true, run: (*m7Suite).m7CleanCase},
		{name: "drift-edited", gen: true, run: func(s *m7Suite, ctx context.Context) error {
			return s.m7DriftCase(ctx, "check-edited", m7FakePath, gen.DriftDiffers, m7AppendEdit)
		}},
		{name: "drift-missing", gen: true, run: func(s *m7Suite, ctx context.Context) error {
			return s.m7DriftCase(ctx, "check-missing", m7PlansPath, gen.DriftMissing, m7Remove)
		}},
		{name: "drift-third-party", gen: true, run: func(s *m7Suite, ctx context.Context) error {
			return s.m7DriftCase(ctx, "check-third-party", m7QueriesPath, gen.DriftThirdParty, m7AppendEdit)
		}},
		{name: "open-hole", gen: true, run: (*m7Suite).m7OpenHoleCase},
	}
}

// m7AppendEdit appends the hand edit to the file at full.
func m7AppendEdit(full string) error {
	body, err := readFile(full)
	if err != nil {
		return err
	}
	return writeFile(full, append(body, m7HandEdit...), fileMode)
}

// m7Remove deletes the file at full.
func m7Remove(full string) error {
	if err := os.Remove(full); err != nil {
		return fmt.Errorf("remove %s: %w", full, err)
	}
	return nil
}

func (s *m7Suite) m7CleanCase(ctx context.Context) error {
	repo, err := s.m7Filled(ctx, "check-clean")
	if err != nil {
		return err
	}
	return m7JudgeCheck(s.m7Check(ctx, repo.dir), 0, []string{m7CheckClean})
}

// m7DriftCase edits path in a filled repository of its own, requires the
// check to name the drift, then runs the fix the line names and requires
// the check clean, so the line is proven to name its own repair.
func (s *m7Suite) m7DriftCase(ctx context.Context, name, path, cause string, edit func(full string) error) error {
	repo, err := s.m7Filled(ctx, name)
	if err != nil {
		return err
	}
	if err := edit(filepath.Join(repo.dir, filepath.FromSlash(path))); err != nil {
		return err
	}
	if err := m7JudgeCheck(s.m7Check(ctx, repo.dir), 1, []string{m7DriftLine(path, cause)}); err != nil {
		return err
	}
	if err := success(s.m5Golaw(ctx, repo.dir, m7GenCommand)); err != nil {
		return fmt.Errorf("repair the drift of %s with golaw gen: %w", path, err)
	}
	return m7JudgeCheck(s.m7Check(ctx, repo.dir), 0, []string{m7CheckClean})
}

func (s *m7Suite) m7OpenHoleCase(ctx context.Context) error {
	repo, err := s.m7Generated(ctx, "check-open")
	if err != nil {
		return err
	}
	ids, err := m7FillIDs(s.root)
	if err != nil {
		return err
	}
	lines := make([]string, 0, len(ids))
	for _, id := range ids {
		lines = append(lines, m7HoleLine(id))
	}
	return m7JudgeCheck(s.m7Check(ctx, repo.dir), 1, lines)
}

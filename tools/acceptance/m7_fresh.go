package main

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
)

// The fresh repository rows of M7.13 rows 1 and 6: the gate values of
// M7.10.1, the marker and the variables of the duplication measure, the
// profile the pipeline's test shape writes and the commit that records the
// third party outputs before the first golaw new.
const (
	m7FreshCategory   = "m7-fresh"
	m7MinCoverage     = 80.0
	m7MaxDuplication  = 1.5
	m7DupMarker       = "M7_DUPLICATION"
	m7DupRepoKey      = "GOLAW_M7_DUP_REPO"
	m7DupBaseKey      = "GOLAW_M7_DUP_BASE"
	m7Profile         = "profile.cov"
	m7GenerateMessage = "build: generate the third party outputs"
)

var (
	m7PercentPattern = regexp.MustCompile(`(\d+(?:\.\d+)?)%`)
	m7DupPattern     = regexp.MustCompile(`(?m)^(?:\s*\S+_test\.go:\d+:\s*)?` + m7DupMarker + ` (\S+)$`)
)

// m7FreshCases are the rows of the fresh repository; each runs golaw gen,
// so a machine without the pinned generators skips them with that cause.
func m7FreshCases() []m7Case {
	return []m7Case{
		{name: "ci-green", gen: true, run: (*m7Suite).m7CIGreen},
		{name: "new-code", gen: true, run: (*m7Suite).m7NewCodeCase},
		{name: "duplication", gen: true, run: (*m7Suite).m7DuplicationCase},
		{name: "holes-open", gen: true, run: (*m7Suite).m7HolesOpenCase},
		{name: "rendering-digest", gen: true, run: (*m7Suite).m7DigestCase},
	}
}

// m7Generated adopts the fixture contracts, commits the third party outputs
// golaw gen writes, records that commit as the base of the new code
// (M7.13 row 1) and writes every kind over it, holes open.
func (s *m7Suite) m7Generated(ctx context.Context, name string) (m7Repo, error) {
	repo, err := s.m7Repository(ctx, name, false)
	if err != nil {
		return m7Repo{}, err
	}
	if genErr := s.m7GenerateOutputs(ctx, repo.dir, name); genErr != nil {
		return m7Repo{}, genErr
	}
	if repo.base, err = s.m7Head(ctx, repo.dir); err != nil {
		return m7Repo{}, err
	}
	if err := s.m7NewAll(ctx, repo.dir); err != nil {
		return m7Repo{}, err
	}
	if err := s.m7Tidy(ctx, repo.dir); err != nil {
		return m7Repo{}, err
	}
	return repo, s.m6Commit(ctx, repo.dir, m7NewMessage)
}

// m7GenerateOutputs completes the tools module, runs golaw gen, requires
// what its outputs import and commits them.
func (s *m7Suite) m7GenerateOutputs(ctx context.Context, dir, name string) error {
	if err := s.m7Tidy(ctx, filepath.Join(dir, m7ToolsDir)); err != nil {
		return err
	}
	if err := success(s.m5Golaw(ctx, dir, m7GenCommand)); err != nil {
		return fmt.Errorf("generate the third party outputs of %s: %w", name, err)
	}
	if err := s.m7Tidy(ctx, dir); err != nil {
		return err
	}
	return s.m6Commit(ctx, dir, m7GenerateMessage)
}

// m7Filled is a generated repository with every recorded fill applied and
// committed.
func (s *m7Suite) m7Filled(ctx context.Context, name string) (m7Repo, error) {
	repo, err := s.m7Generated(ctx, name)
	if err != nil {
		return m7Repo{}, err
	}
	fills, err := m7Fills(s.root)
	if err != nil {
		return m7Repo{}, err
	}
	if err := m7ApplyFills(repo.dir, fills); err != nil {
		return m7Repo{}, err
	}
	if err := success(s.m3Make(ctx, repo.dir, "fmt")); err != nil {
		return m7Repo{}, fmt.Errorf("format the fills of %s: %w", name, err)
	}
	return repo, s.m6Commit(ctx, repo.dir, m7FillMessage)
}

// m7FreshRepo builds the filled fresh repository once per run, because
// three rows read it and each build runs the generators and every kind.
func (s *m7Suite) m7FreshRepo(ctx context.Context) (m7Repo, error) {
	if !s.gen.filledTried {
		s.gen.filledTried = true
		repo, err := s.m7Filled(ctx, "fresh")
		s.gen.filled, s.gen.filledErr = &repo, err
	}
	return *s.gen.filled, s.gen.filledErr
}

func (s *m7Suite) m7CIGreen(ctx context.Context) error {
	repo, err := s.m7FreshRepo(ctx)
	if err != nil {
		return err
	}
	made, err := s.m7MakeCI(ctx, repo)
	if err != nil {
		return err
	}
	if err := success(made); err != nil {
		return fmt.Errorf("make ci of the filled repository: %w", err)
	}
	return nil
}

// m7NewCodeCase writes the profile in the pipeline's shape, without
// -coverpkg (M7.5), and measures it against the commit before the first
// golaw new.
func (s *m7Suite) m7NewCodeCase(ctx context.Context) error {
	repo, err := s.m7FreshRepo(ctx)
	if err != nil {
		return err
	}
	profile := command{program: s.goTool, args: []string{m7TestTarget, "-coverprofile", m7Profile, "./..."}, dir: repo.dir, env: s.env}
	if err := success(s.execute(ctx, profile)); err != nil {
		return fmt.Errorf("write the coverage profile of the filled repository: %w", err)
	}
	return m7JudgeCoverage(s.m5Golaw(ctx, repo.dir, "coverage", "--new-code", "--input", m7Profile, "--base", repo.base))
}

// m7Percent answers the first percentage of text.
func m7Percent(text string) (float64, bool) {
	match := m7PercentPattern.FindStringSubmatch(text)
	if match == nil {
		return 0, false
	}
	value, err := strconv.ParseFloat(match[1], 64)
	return value, err == nil
}

// m7JudgeCoverage requires a measured report at the M7.10.1 floor.
func m7JudgeCoverage(got result) error {
	if err := success(got); err != nil {
		return fmt.Errorf("golaw coverage --new-code: %w", err)
	}
	value, ok := m7Percent(got.stdout)
	if !ok {
		return fmt.Errorf("golaw coverage --new-code printed no percentage: %q: %w", got.stdout, ErrAcceptance)
	}
	if value < m7MinCoverage {
		return fmt.Errorf("coverage on new code %.1f%% is under the floor of %.0f%%: %w", value, m7MinCoverage, ErrAcceptance)
	}
	return nil
}

// m7DuplicationCase runs the test only measure of contract 3.4 rule 4 over
// the fresh repository, as the M2 timing row runs its measurement test.
func (s *m7Suite) m7DuplicationCase(ctx context.Context) error {
	repo, err := s.m7FreshRepo(ctx)
	if err != nil {
		return err
	}
	got := s.execute(ctx, command{
		program: s.goTool, args: []string{m7TestTarget, "-run", "^TestM7Duplication$", "-count=1", "-v", "./tools/acceptance/"},
		dir: s.root, env: append(slices.Clone(s.env), m7DupRepoKey+"="+repo.dir, m7DupBaseKey+"="+repo.base), timeout: m2MeasurementTimeout,
	})
	if err := success(got); err != nil {
		return fmt.Errorf("measure the duplication of the filled repository: %w", err)
	}
	return m7JudgeDuplication(got.stdout)
}

// m7JudgeDuplication reads the last marker the measurement test logged and
// requires it under the A50 threshold.
func m7JudgeDuplication(output string) error {
	matches := m7DupPattern.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return fmt.Errorf("the duplication measure logged no %s line: %w", m7DupMarker, ErrAcceptance)
	}
	text := matches[len(matches)-1][1]
	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return fmt.Errorf("the duplication marker %q is not a number: %w", text, ErrAcceptance)
	}
	if value >= m7MaxDuplication {
		return fmt.Errorf("duplication on new code %.2f%% reaches the threshold of %.1f%%: %w", value, m7MaxDuplication, ErrAcceptance)
	}
	return nil
}

// m7HolesOpenCase runs make -k ci and golaw gen --check before the fills.
func (s *m7Suite) m7HolesOpenCase(ctx context.Context) error {
	repo, err := s.m7Generated(ctx, "holes")
	if err != nil {
		return err
	}
	ids, err := m7FillIDs(s.root)
	if err != nil {
		return err
	}
	made, err := s.m7MakeCI(ctx, repo, "-k")
	if err != nil {
		return err
	}
	return m7JudgeHolesOpen(made, s.m7Check(ctx, repo.dir), ids)
}

// m7FillIDs are the hole ids of the recorded fills, every hole the
// commands open.
func m7FillIDs(root string) ([]string, error) {
	fills, err := m7Fills(root)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(fills))
	for _, fill := range fills {
		ids = append(ids, fill.ID)
	}
	return ids, nil
}

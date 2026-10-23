package main

import (
	"context"
	"path/filepath"
)

// The profile rows, the two trees of the synthetic fixture and the file
// only the red tree holds.
const (
	m5ProfileCategory = "m5-profile"
	m5Red             = "tools/acceptance/testdata/m5/profile/red"
	m5Corrected       = "tools/acceptance/testdata/m5/profile/corrected"
	m5RedOnly         = "AGENT-NOTES.md"
)

// m5Setup is the state every lifecycle row starts from: the repository,
// the local source, the output of the first install and the committed pin.
type m5Setup struct {
	repo, source string
	installed    result
	pin          []byte
}

func m5ProfileCases() []m5Case {
	return []m5Case{
		{name: "fixture-fails-with-profile", run: (*suite).m5FailsWithProfile},
		{name: "fixture-passes-without", run: (*suite).m5PassesWithout},
		{name: "fixture-passes-corrected", run: (*suite).m5PassesCorrected},
		{name: "digest-mismatch", run: (*suite).m5DigestMismatch},
		{name: "schema-mismatch", run: (*suite).m5SchemaMismatch},
		{name: "stale-bundle", run: (*suite).m5StaleBundle},
	}
}

// m5RedTree is a repository adopted with or without the fixture profile,
// the red tree committed over it.
func (s *suite) m5RedTree(ctx context.Context, name string, profiled bool) (string, error) {
	if profiled {
		source, err := s.m5Source(ctx)
		if err != nil {
			return "", err
		}
		if _, err := s.m5Install(ctx, source, m5Tag); err != nil {
			return "", err
		}
	}
	repo, err := s.m5Repository(ctx, name, profiled)
	if err != nil {
		return "", err
	}
	if stepErr := m5Overlay(s.root, m5Red, repo); stepErr != nil {
		return "", stepErr
	}
	return repo, s.gitSteps(ctx, repo, []string{gitAdd, "."}, commitArgs("test: the red tree of the fixture profile"))
}

// m5CleanRepository is a repository the profile and the core both find
// clean: the bundle installed from the local source, the adoption under
// the profile and the corrected tree committed.
func (s *suite) m5CleanRepository(ctx context.Context, name string) (m5Setup, error) {
	source, err := s.m5Source(ctx)
	if err != nil {
		return m5Setup{}, err
	}
	installed, err := s.m5Install(ctx, source, m5Tag)
	if err != nil {
		return m5Setup{}, err
	}
	repo, err := s.m5Repository(ctx, name, true)
	if err != nil {
		return m5Setup{}, err
	}
	if overlayErr := m5Overlay(s.root, m5Corrected, repo); overlayErr != nil {
		return m5Setup{}, overlayErr
	}
	commit := commitArgs("test: the corrected tree of the fixture profile")
	if gitErr := s.gitSteps(ctx, repo, []string{gitAdd, "."}, commit); gitErr != nil {
		return m5Setup{}, gitErr
	}
	pin, err := readFile(filepath.Join(repo, policyFile))
	return m5Setup{repo: repo, source: source, installed: installed, pin: pin}, err
}

func (s *suite) m5FailsWithProfile(ctx context.Context) error {
	repo, err := s.m5RedTree(ctx, "profiled", true)
	if err != nil {
		return err
	}
	doc, code, err := s.m3Scan(ctx, repo)
	if err != nil {
		return err
	}
	return m5JudgeProfiled(doc, code)
}

func (s *suite) m5PassesWithout(ctx context.Context) error {
	repo, err := s.m5RedTree(ctx, "core", false)
	if err != nil {
		return err
	}
	doc, code, err := s.m3Scan(ctx, repo)
	if err != nil {
		return err
	}
	return m5JudgeCore(doc, code)
}

// m5PassesCorrected corrects the red tree under the profile: the files
// rewritten, the forbidden path removed, the same scan clean.
func (s *suite) m5PassesCorrected(ctx context.Context) error {
	repo, err := s.m5RedTree(ctx, "corrected", true)
	if err != nil {
		return err
	}
	if overlayErr := m5Overlay(s.root, m5Corrected, repo); overlayErr != nil {
		return overlayErr
	}
	steps := [][]string{{"rm", m2Quiet, m5RedOnly}, {gitAdd, "."}, commitArgs("fix: correct the fixture tree")}
	if gitErr := s.gitSteps(ctx, repo, steps...); gitErr != nil {
		return gitErr
	}
	doc, code, err := s.m3Scan(ctx, repo)
	if err != nil {
		return err
	}
	return m5JudgeCorrected(doc, code)
}

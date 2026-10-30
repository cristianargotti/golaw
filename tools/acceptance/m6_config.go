package main

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	scanreport "github.com/cristianargotti/golaw/internal/report"
	"github.com/cristianargotti/golaw/internal/surfacefix"
)

// The category of row (7) of M6.10.
const m6ConfigCategory = "m6-config"

// m6Lined identifies one finding to its line, so a configuration that
// moved a finding instead of hiding it is still a change.
type m6Lined struct {
	path, check, code string
	line              int
}

// m6ConfigCases is the one row of a repository configuration; it runs the
// surface tools, so it needs them.
func m6ConfigCases() []m6Case {
	return []m6Case{{name: "repository-config-ignored", tools: true, run: (*m6Suite).m6ConfigIgnored}}
}

// m6Findings counts the findings on the red twin's own paths: a finding a
// linter raises on a configuration file itself, such as yamllint on a
// hadolint configuration, is outside the comparison.
func m6Findings(doc scanreport.Document, paths map[string][]byte) map[m6Lined]int {
	out := make(map[m6Lined]int)
	for i := range doc.Findings {
		f := &doc.Findings[i]
		if _, twin := paths[f.Path]; twin {
			out[m6Lined{f.Path, f.Check, f.Code, f.Line}]++
		}
	}
	return out
}

// m6LintedFindings decodes the findings of a lint run that found some.
func m6LintedFindings(got result, which string, paths map[string][]byte) (map[m6Lined]int, error) {
	if got.err != nil {
		return nil, fmt.Errorf(m6RunCandidate, got.err)
	}
	if got.code != 1 {
		return nil, fmt.Errorf("the %s lint exit %d %q, want 1 for findings: %w", which, got.code, strings.TrimSpace(got.stderr), ErrAcceptance)
	}
	doc, err := m6Document(got.stdout)
	if err != nil {
		return nil, fmt.Errorf("the %s lint: %w", which, err)
	}
	return m6Findings(doc, paths), nil
}

// m6JudgeSame requires the configured repository to raise on the twin's
// paths exactly the findings the plain one raises.
func m6JudgeSame(plain, configured result, paths map[string][]byte) error {
	want, err := m6LintedFindings(plain, "plain", paths)
	if err != nil {
		return err
	}
	got, err := m6LintedFindings(configured, "configured", paths)
	if err != nil {
		return err
	}
	if len(want) == 0 {
		return fmt.Errorf("the red twin raised no lint finding: nothing to compare: %w", ErrAcceptance)
	}
	if maps.Equal(want, got) {
		return nil
	}
	keys := slices.Collect(maps.Keys(want))
	for k := range got {
		if _, seen := want[k]; !seen {
			keys = append(keys, k)
		}
	}
	slices.SortFunc(keys, m6CompareLined)
	for _, k := range keys {
		if want[k] != got[k] {
			return fmt.Errorf("a repository configuration changed %s:%d %s %s from %d to %d: %w", k.path, k.line, k.check, k.code, want[k], got[k], ErrAcceptance)
		}
	}
	return nil
}

// m6CompareLined orders keys so a failure always names the same key first.
func m6CompareLined(a, b m6Lined) int {
	return cmp.Or(cmp.Compare(a.path, b.path), cmp.Compare(a.line, b.line), cmp.Compare(a.check, b.check), cmp.Compare(a.code, b.code))
}

// m6ConfigIgnored lints the red twin, then the same twin with a repository
// configuration of every hook linter beside it that disables a rule the
// twin trips, and requires the same findings, since the kit's tools run
// with repository configuration lookup disabled (M6.4.1).
func (s *m6Suite) m6ConfigIgnored(ctx context.Context) error {
	twin, err := surfacefix.Files(surfacefix.Red)
	if err != nil {
		return fmt.Errorf("read the surface twin: %w", err)
	}
	plain, err := s.m6Twin(ctx, "config-plain", surfacefix.Red, nil)
	if err != nil {
		return err
	}
	configs, err := m6Fixture(s.root, "configs")
	if err != nil {
		return err
	}
	merged := maps.Clone(twin)
	maps.Copy(merged, configs)
	configured, err := s.m6Repository(ctx, "config-repository", merged, nil)
	if err != nil {
		return err
	}
	return m6JudgeSame(s.m6Report(ctx, plain, true), s.m6Report(ctx, configured, true), twin)
}

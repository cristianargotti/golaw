package main

import (
	"context"
	"fmt"
	"io"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

const (
	m2StagedCategory    = "m2-staged"
	m2LifecycleCategory = "m2-lifecycle"
	m2RungCategory      = "m2-rung"
	m2Preparation       = "m2-preparation"
	m2MeasureKey        = "GOLAW_M2_MEASURE"
	m2CompleteMarker    = "M2_MEASUREMENTS_COMPLETE"
	m2SetupCleanCase    = "setup-clean"
	// A passing M2 row ran the candidate of this run, whose build falls
	// back to the public module proxy, with the tools setup-clean installed
	// from it, so the M1 detail "verified offline" would misstate it.
	m2Verified = "verified against the candidate"
)

func (s *suite) collectM2(ctx context.Context) ([]row, map[string]rowEvidence) {
	version, err := s.m2TimingVersion(ctx, m2GitCommand)
	if err != nil {
		return s.m2PreparationFailure("git-version", err)
	}
	catalogs := []struct {
		category string
		names    []string
	}{
		{m2StagedCategory, m2StagedNames()},
		{m2LifecycleCategory, m2LifecycleNames()},
		{m2RungCategory, m2NativeNames()},
	}
	var rows []row
	evidence := make(map[string]rowEvidence)
	for _, catalog := range catalogs {
		var clean row
		if catalog.category == m2LifecycleCategory {
			clean = s.m2CollectedRow(ctx, catalog.category, m2SetupCleanCase)
		}
		for _, name := range catalog.names {
			item := clean
			if name != m2SetupCleanCase {
				item = s.m2CollectedRow(ctx, catalog.category, name)
			}
			rows = append(rows, item)
			evidence[item.category+"/"+name] = rowEvidence{Agent: m2GitCommand, Version: version, Binary: s.digest}
		}
	}
	return rows, evidence
}

func (s *suite) m2CollectedRow(ctx context.Context, category, name string) row {
	scope := s.m2Scope(name)
	switch category {
	case m2StagedCategory:
		return measuredM2(category, name, scope.stagedM2(ctx, name))
	case m2LifecycleCategory:
		return measuredM2(category, name, scope.lifecycleM2(ctx, name))
	default:
		item := scope.nativeM2(ctx, name)
		item.category = category
		return item
	}
}

func (s *suite) m2PreparationFailure(name string, err error) ([]row, map[string]rowEvidence) {
	item := measuredM2(m2Preparation, name, err)
	return []row{item}, map[string]rowEvidence{
		item.category + "/" + name: {Agent: m2GitCommand, Binary: s.digest},
	}
}

func (s *suite) measurementM2(ctx context.Context) row {
	env := slices.DeleteFunc(slices.Clone(s.env), func(entry string) bool {
		key, _, _ := strings.Cut(entry, "=")
		return strings.EqualFold(key, m2MeasureKey)
	})
	got := s.execute(ctx, command{
		program: s.goTool,
		args:    []string{"test", "-race", "-timeout=45m", "-run", "^TestM2Measurements$", "-count=1", "-v", "./tools/acceptance/"},
		dir:     s.root, env: append(env, m2MeasureKey+"=1"), timeout: m2MeasurementTimeout,
	})
	err := success(got)
	if err == nil && !m2MeasureComplete(got.stdout) {
		err = fmt.Errorf("native measurements lack their completion marker: %w", ErrAcceptance)
	}
	return measuredM2("m2-timing", "native", err)
}

// measuredM2 is the row measured builds, with the detail of a passing M2
// row.
func measuredM2(category, name string, err error) row {
	got := measured(category, name, err)
	if err == nil {
		got.detail = m2Verified
	}
	return got
}

func m2MeasureComplete(output string) bool {
	marker := regexp.MustCompile(`^(?:\s*m2_measurement_internal_test\.go:\d+:\s*)?` + m2CompleteMarker + `$`)
	for line := range strings.SplitSeq(strings.ReplaceAll(output, "\r\n", "\n"), "\n") {
		if marker.MatchString(line) {
			return true
		}
	}
	return false
}

func (s *suite) runAcceptance(ctx context.Context, out io.Writer, required, onlyM2 bool) int {
	if onlyM2 && !required {
		rows, evidence := s.m2AcceptanceRows(ctx)
		return m2ReportCode(reportEvidence(out, rows, "", s.digest, evidence))
	}
	if err := s.prepare(ctx); err != nil {
		return setupFailure(out, err)
	}
	rows, version := s.collect(ctx, required)
	var evidence map[string]rowEvidence
	if required {
		candidate := newSuite(s.root, filepath.Join(s.base, "m2"), s.execute, s.lookup)
		var additional []row
		additional, evidence = candidate.m2AcceptanceRows(ctx)
		rows = append(rows, additional...)
		// A release requires the M3 rows as it requires the M2 ones (A23).
		m3Rows, m3Evidence := newSuite(s.root, filepath.Join(s.base, "m3"), s.execute, s.lookup).m3AcceptanceRows(ctx)
		rows = append(rows, m3Rows...)
		maps.Copy(evidence, m3Evidence)
		m4Rows, m4Evidence := s.collectM4(ctx, true)
		rows = append(rows, m4Rows...)
		maps.Copy(evidence, m4Evidence)
	}
	return m2ReportCode(reportEvidence(out, rows, version, s.digest, evidence))
}

func (s *suite) m2AcceptanceRows(ctx context.Context) ([]row, map[string]rowEvidence) {
	execute := s.execute
	s.execute = func(ctx context.Context, c command) result {
		if c.timeout == 0 {
			c.timeout = m2ProcessTimeout
		}
		return execute(ctx, c)
	}
	if err := s.prepareM2(ctx); err != nil {
		return s.m2PreparationFailure(candidateRow, err)
	}
	return s.collectM2(ctx)
}

func m2ReportCode(passed bool) int {
	if passed {
		return 0
	}
	return 1
}

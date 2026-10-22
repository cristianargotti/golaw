package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestM3BirthMakefileIsMeasuredWithinTheSubset(t *testing.T) {
	t.Parallel()
	birth := string(m3BirthTree(t, []string{"go"})["Makefile"])
	for _, tc := range []struct {
		name, makefile string
		finding        string
	}{
		{"red as born, no include", birth, "deps-age"},
		{"red include accepted over colliding targets", m3Included(birth), "lint"},
		{"near targets renamed and the include accepted", m3Included(m3Renamed(birth)), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			files := m3BirthTree(t, []string{"go"})
			files["Makefile"] = []byte(tc.makefile)
			result := m3Shape(t, policyfiles.Makefile(), files, []string{"go"}, nil)
			if result.Status.Kind != model.StatusMeasured || model.IsOutsideSubset(result.Status) {
				t.Fatalf("status = %+v: the birth Makefile must be read within the subset", result.Status)
			}
			if tc.finding == "" {
				if len(result.Findings) != 0 {
					t.Fatalf("near tree has findings: %+v", result.Findings)
				}
				return
			}
			if !slices.ContainsFunc(result.Findings, func(f model.Finding) bool { return strings.Contains(f.Message, tc.finding) }) {
				t.Fatalf("no finding names %q: %+v", tc.finding, result.Findings)
			}
		})
	}
}

package main

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestM3BirthGetsTheCiAndDepsAgeTargetsThroughTheInclude(t *testing.T) {
	t.Parallel()
	birth := string(m3BirthTree(t, []string{"go"})["Makefile"])
	for _, check := range []model.Check{policyfiles.CITarget(), policyfiles.DepsAgeConfig()} {
		for _, tc := range []struct {
			name, makefile string
			red            bool
		}{
			{"red as born", birth, true},
			{"near include accepted", m3Included(m3Renamed(birth)), false},
		} {
			files := m3BirthTree(t, []string{"go"})
			files["Makefile"] = []byte(tc.makefile)
			result := m3Shape(t, check, files, []string{"go"}, nil)
			if result.Status.Kind != model.StatusMeasured || (len(result.Findings) > 0) != tc.red {
				t.Fatalf("%s %s: status %+v, findings %+v", check.Spec().ID, tc.name, result.Status, result.Findings)
			}
		}
	}
}

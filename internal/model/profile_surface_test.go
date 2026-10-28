package model_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

func TestSectionAnswersTheSurfaceSectionsOfABundle(t *testing.T) {
	t.Parallel()
	analyzer := &model.AnalyzerPolicy{Exclusions: []string{"**/*_test.go"}}
	snapshot := &model.SonarWay{Read: "2026-10-27", Rules: []model.SonarWayRule{{Language: "js", Key: "S2681", Severity: "MAJOR"}}}
	tests := map[string]struct {
		data    *model.ProfileData
		section string
		want    bool
	}{
		"an analyzer.yml":                   {data: &model.ProfileData{Analyzer: analyzer}, section: model.SectionAnalyzer, want: true},
		"a sonar-way snapshot":              {data: &model.ProfileData{SonarWay: snapshot}, section: model.SectionSonarWay, want: true},
		"no analyzer.yml (near miss)":       {data: &model.ProfileData{SonarWay: snapshot}, section: model.SectionAnalyzer, want: false},
		"no sonar-way snapshot (near miss)": {data: &model.ProfileData{Analyzer: analyzer}, section: model.SectionSonarWay, want: false},
		"core only, no bundle (near miss)":  {data: nil, section: model.SectionAnalyzer, want: false},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := tc.data.Section(tc.section); got != tc.want {
				t.Errorf("Section(%q) = %v, want %v", tc.section, got, tc.want)
			}
		})
	}
	if model.SectionAnalyzer != "analyzer" || model.SectionSonarWay != "sonar-way" {
		t.Errorf("section names = %q, %q", model.SectionAnalyzer, model.SectionSonarWay)
	}
}

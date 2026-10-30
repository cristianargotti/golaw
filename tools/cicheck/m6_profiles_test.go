package cicheck_test

import (
	"strings"
	"testing"
)

const m6ProfilesPath = "docs/profiles.md"

func m6ProfileStatements() []string {
	return []string{
		"## Analyzer scope and Sonar way snapshots",
		"A bundle may carry two more data files, both data and never code, both covered by the bundle digest.",
		"analyzer.yml describes the remote analyzer of the platform that adopts the profile",
		"coverage_profile names how the pipeline's test step writes its report",
		"it wins over the repository's own sonar-project.properties, because a platform pipeline may delete that file before it scans",
		"A company's exclusion list lives here, in its private profile, and never in the public tree.",
		"sonar-way/<language>.yml is a snapshot of the active Sonar way rules of one server for one language, with the day it was read",
		"The public kit ships only the map derived from public sources",
		"A snapshot is as fresh as its read date",
		"A profile's remote gate is whatever the adopting platform runs.",
	}
}

func TestM6ProfilesDescribeTheTwoDataFiles(t *testing.T) {
	t.Parallel()
	text := string(kitBytes(t, m6ProfilesPath))
	statements := m6ProfileStatements()
	forbidden := []string{"analyzer.yml runs", "sonar-way snapshot is fetched"}
	if !m4TextOK(text, statements, forbidden) {
		for _, statement := range statements {
			if !strings.Contains(text, statement) {
				t.Errorf("%s lacks %q", m6ProfilesPath, statement)
			}
		}
		t.FailNow()
	}
	for _, red := range []string{statements[3], statements[5], statements[9]} {
		if m4TextOK(strings.Replace(text, red, "", 1), statements, forbidden) {
			t.Errorf("red: the page without %q passed", red)
		}
	}
	if m4TextOK(text+"\nThe sonar-way snapshot is fetched by the hook.\n", statements, forbidden) {
		t.Error("red: a fetched snapshot passed")
	}
}

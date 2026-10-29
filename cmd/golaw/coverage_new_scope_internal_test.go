package main

import (
	"maps"
	"path/filepath"
	"strings"
	"testing"
)

// flowSource is a package the pipeline exclusions drop: its return line 4
// is a new uncovered line.
const flowSource = "package integration\n\nfunc Flow() int {\n\treturn 1\n}\n"

func flowBlock() string { return newCodeModule + "/integration/flow.go:3.17,5.2 1 0" }

func withFlow(extra map[string]string) map[string]string {
	out := map[string]string{"integration/flow.go": flowSource}
	maps.Copy(out, extra)
	return out
}

func TestCoverageNewCodeReadsTheAnalyzerScope(t *testing.T) {
	t.Parallel()
	properties := map[string]string{"sonar-project.properties": "sonar.exclusions=integration/**\n"}
	placeholder := map[string]string{"sonar-project.properties": "sonar.exclusions=${EXCLUDED}\n"}
	for _, tc := range []struct {
		name string
		c    newCodeCase
		code int
		want string
	}{
		{"red-no-source-the-whole-module-counts", newCodeCase{}, exitFindings, "integration/flow.go: 0/1 covered, uncovered lines 4"},
		{"near-golaw-yml-exclusions", newCodeCase{policy: "surface:\n  analyzer: {exclusions: [integration/**]}\n"}, exitClean, "analyzer scope policy"},
		{"near-sonar-project-properties", newCodeCase{extra: properties}, exitClean, "analyzer scope properties"},
		{"near-the-synthetic-profile", newCodeCase{profiled: true}, exitClean, "analyzer scope profile"},
		{"red-a-pinned-source-that-is-absent", newCodeCase{policy: "surface:\n  analyzer: {from: properties}\n"}, exitFailure, "analyzer scope source absent"},
		{"red-a-placeholder-in-the-properties", newCodeCase{extra: placeholder}, exitFailure, "sonar.exclusions"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := tc.c
			c.extra = withFlow(c.extra)
			f := newCodeFixture(t, c)
			report := f.write(t, "profile.cov", newCodeProfile(4, flowBlock()), true)
			code, text := f.run(t, "--new-code", "--input", report, "--base", "base")
			if code != tc.code || !strings.Contains(text, tc.want) {
				t.Fatalf("exit = %d, output = %q", code, text)
			}
		})
	}
}

func TestCoverageNewCodeReadsTheReportTheScopeNames(t *testing.T) {
	t.Parallel()
	f := newCodeFixture(t, newCodeCase{policy: "surface:\n  analyzer: {coverage_report: build/parity.cov}\n"})
	f.write(t, "build/parity.cov", newCodeProfile(4), true)
	code, text := f.run(t, "--new-code", "--base", "base")
	if code != exitClean || !strings.Contains(text, "report "+filepath.Join(f.repo.Root, "build", "parity.cov")) {
		t.Fatalf("exit = %d, output = %q", code, text)
	}
}

func TestCoverageNewCodeNeedsTheModule(t *testing.T) {
	t.Parallel()
	f := newCodeFixture(t, newCodeCase{extra: map[string]string{"go.mod": "go 1.27\n"}})
	report := f.write(t, "profile.cov", newCodeProfile(4), true)
	code, text := f.run(t, "--new-code", "--input", report, "--base", "base")
	if code != exitFailure || !strings.Contains(text, "go.mod names no module") {
		t.Fatalf("exit = %d, output = %q", code, text)
	}
}

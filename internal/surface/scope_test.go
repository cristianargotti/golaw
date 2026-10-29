package surface_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/surface"
)

// pipelineExclusions is the exclusion list a company's private profile
// carries in its analyzer.yml (M6.3.3), as generic data.
var pipelineExclusions = []string{
	"**/*_test.go", "integration/**", "**/gen/**", "**/testdata/**", "examples/*",
}

// policyOf is the compiled golaw.yml block of a test: from and the lists.
func policyOf(from string, exclusions ...string) model.AnalyzerPolicy {
	a := model.DefaultSurface().Analyzer
	a.From, a.Exclusions = from, exclusions
	return a
}

func TestResolveFollowsThePrecedenceOfAuto(t *testing.T) {
	t.Parallel()
	profile := &model.AnalyzerPolicy{Exclusions: pipelineExclusions, Coverage: model.CoverProfile{CoverPkg: true}}
	props := []byte("sonar.exclusions=**/*_test.go\n")
	tests := map[string]struct {
		in             surface.ScopeInput
		source, origin string
	}{
		"the profile wins over the properties and golaw.yml": {
			in:     surface.ScopeInput{Policy: policyOf(model.FromAuto, "vendor/**"), Profile: profile, Profiled: "surface", Properties: props, HasProps: true},
			source: model.FromProfile, origin: "surface/analyzer.yml",
		},
		"the properties win over golaw.yml": {
			in:     surface.ScopeInput{Policy: policyOf(model.FromAuto, "vendor/**"), Properties: props, HasProps: true},
			source: model.FromProperties, origin: "sonar-project.properties",
		},
		"golaw.yml when it names a list": {
			in:     surface.ScopeInput{Policy: policyOf(model.FromAuto, "vendor/**")},
			source: model.FromPolicy, origin: "golaw.yml",
		},
		"none when nothing describes the pipeline": {in: surface.ScopeInput{Policy: policyOf(model.FromAuto)}, source: surface.SourceNone},
		"an empty from reads as auto":              {in: surface.ScopeInput{Policy: policyOf("")}, source: surface.SourceNone},
		"a pinned policy wins over the properties": {
			in:     surface.ScopeInput{Policy: policyOf(model.FromPolicy), Properties: props, HasProps: true},
			source: model.FromPolicy, origin: "golaw.yml",
		},
		"a pinned properties file wins over the profile": {
			in:     surface.ScopeInput{Policy: policyOf(model.FromProperties), Profile: profile, Profiled: "surface", Properties: props, HasProps: true},
			source: model.FromProperties, origin: "sonar-project.properties",
		},
		"a pinned profile": {
			in:     surface.ScopeInput{Policy: policyOf(model.FromProfile), Profile: profile, Profiled: "surface"},
			source: model.FromProfile, origin: "surface/analyzer.yml",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := surface.Resolve(tc.in)
			if err != nil || got.Source != tc.source || got.Origin != tc.origin {
				t.Fatalf("Resolve = %+v, %v, want source %q from %q", got, err, tc.source, tc.origin)
			}
		})
	}
}

func TestResolveRefusesAPinnedSourceThatIsAbsent(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		in   surface.ScopeInput
		want string
	}{
		"properties without the file": {
			in:   surface.ScopeInput{Policy: policyOf(model.FromProperties)},
			want: "surface.analyzer.from is properties and the repository root holds no sonar-project.properties",
		},
		"a profile when none is pinned": {
			in:   surface.ScopeInput{Policy: policyOf(model.FromProfile)},
			want: "surface.analyzer.from is profile and no profile is pinned",
		},
		"a profile without analyzer.yml": {
			in:   surface.ScopeInput{Policy: policyOf(model.FromProfile), Profiled: "fixture"},
			want: "surface.analyzer.from is profile and the pinned profile fixture carries no analyzer.yml",
		},
		"a source golaw does not know": {
			in:   surface.ScopeInput{Policy: policyOf("remote")},
			want: `surface.analyzer.from "remote" is not auto, profile, properties or policy`,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := surface.Resolve(tc.in)
			if !errors.Is(err, surface.ErrScopeSource) || err.Error() != "analyzer scope source absent: "+tc.want {
				t.Fatalf("Resolve = %v, want ErrScopeSource naming %q", err, tc.want)
			}
		})
	}
}

// propertiesScope resolves a properties file under from: auto.
func propertiesScope(t *testing.T, data string) surface.Scope {
	t.Helper()
	s, err := surface.Resolve(surface.ScopeInput{Policy: policyOf(model.FromAuto), Properties: []byte(data), HasProps: true})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestRoleReadsThePathsAsThePipelineDoes(t *testing.T) {
	t.Parallel()
	pipeline := propertiesScope(t, pipelineProperties)
	withTests := propertiesScope(t, "sonar.sources=.\nsonar.tests=.\nsonar.test.inclusions=**/*_test.go,test/**\nsonar.exclusions=**/*_test.go,test/**\n")
	included := propertiesScope(t, "sonar.sources=./src/\nsonar.inclusions=**/*.ts\nsonar.coverage.exclusions=src/cmd/**\n")
	overlap := propertiesScope(t, "sonar.tests=test\nsonar.test.exclusions=test/fixtures/**\n")
	none, err := surface.Resolve(surface.ScopeInput{Policy: policyOf(model.FromAuto)})
	if err != nil {
		t.Fatal(err)
	}
	tests := map[string]struct {
		scope   surface.Scope
		path    string
		role    surface.Role
		covered bool
	}{
		"main code under the pipeline":                {scope: pipeline, path: "cmd/hub/main.go", role: surface.RoleMain, covered: true},
		"a Go test the pipeline excludes":             {scope: pipeline, path: "cmd/hub/main_test.go", role: surface.RoleOutside},
		"the integration package":                     {scope: pipeline, path: "integration/flow.go", role: surface.RoleOutside},
		"an example at the first level":               {scope: pipeline, path: "examples/api.go", role: surface.RoleOutside},
		"an example one level deeper is main (near)":  {scope: pipeline, path: "examples/deep/api.go", role: surface.RoleMain, covered: true},
		"a generated client":                          {scope: pipeline, path: "internal/gen/api.go", role: surface.RoleOutside},
		"a script test is main code, the M6 case":     {scope: pipeline, path: "test/view/host.test.cjs", role: surface.RoleMain, covered: true},
		"an HTML view is main code":                   {scope: pipeline, path: "views/panel.html", role: surface.RoleMain, covered: true},
		"a test under declared tests":                 {scope: withTests, path: "internal/a/b_test.go", role: surface.RoleTest},
		"a test directory under declared tests":       {scope: withTests, path: "test/view/host.test.cjs", role: surface.RoleTest},
		"main code beside declared tests":             {scope: withTests, path: "internal/a/b.go", role: surface.RoleMain, covered: true},
		"an included source":                          {scope: included, path: "src/app.ts", role: surface.RoleMain, covered: true},
		"a source the inclusions leave out (near)":    {scope: included, path: "src/app.js", role: surface.RoleOutside},
		"a file outside the sources (near)":           {scope: included, path: "lib/app.ts", role: surface.RoleOutside},
		"main code outside the coverage":              {scope: included, path: "src/cmd/main.ts", role: surface.RoleMain},
		"a path both claim is test code":              {scope: overlap, path: "test/a.py", role: surface.RoleTest},
		"a test exclusion falls back to main (near)":  {scope: overlap, path: "test/fixtures/a.py", role: surface.RoleMain, covered: true},
		"no description reads the whole tree as main": {scope: none, path: "scripts/smoke.sh", role: surface.RoleMain, covered: true},
		"no description names no test (near)":         {scope: none, path: "cmd/hub/main_test.go", role: surface.RoleMain, covered: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := tc.scope.Role(tc.path); got != tc.role {
				t.Errorf("Role(%q) = %d, want %d", tc.path, got, tc.role)
			}
			if got := tc.scope.Covered(tc.path); got != tc.covered {
				t.Errorf("Covered(%q) = %v, want %v", tc.path, got, tc.covered)
			}
		})
	}
}

func TestResolveNamesWhatItCannotRead(t *testing.T) {
	t.Parallel()
	s := propertiesScope(t, "sonar.sources=src/*,./cmd/,..,/srv,./\nsonar.exclusions=**/*.{js,ts},integration/**\n"+
		"sonar.go.coverage.reportPaths=a.cov,b.cov\nsonar.tests=${TESTS}\n")
	want := []string{
		"sonar.tests: a ${...} placeholder golaw does not resolve",
		"coverage_report: 2 reports are named, golaw reads only the first",
		`sources: "src/*" is not a relative directory or file golaw reads`,
		`sources: ".." is not a relative directory or file golaw reads`,
		`sources: "/srv" is not a relative directory or file golaw reads`,
		`exclusions: analyzer pattern golaw cannot parse: "**/*.{js" holds a character outside **, * and ?`,
		`exclusions: analyzer pattern golaw cannot parse: "ts}" holds a character outside **, * and ?`,
	}
	if !slices.Equal(s.Unmeasured, want) {
		t.Errorf("Unmeasured = %q, want %q", s.Unmeasured, want)
	}
	if !slices.Equal(s.Sources, []string{"cmd", "."}) || !slices.Equal(s.Exclusions, []string{"integration/**"}) || s.CoverageReport != "a.cov" {
		t.Errorf("scope = %+v, want the readable entries kept", s)
	}
}

func TestResolveFillsTheCoverageDefaults(t *testing.T) {
	t.Parallel()
	policy := policyOf(model.FromAuto)
	policy.CoverageReport = "build/profile.cov"
	none, err := surface.Resolve(surface.ScopeInput{Policy: policy})
	if err != nil || none.CoverageReport != "build/profile.cov" || none.Coverage != (model.CoverProfile{CoverMode: "set"}) {
		t.Fatalf("none = %+v, %v, want the golaw.yml report and mode set", none, err)
	}
	bare, err := surface.Resolve(surface.ScopeInput{})
	if err != nil || bare.CoverageReport != model.DefaultCoverageReport {
		t.Fatalf("an empty input = %+v, %v, want the default report", bare, err)
	}
	profiled, err := surface.Resolve(surface.ScopeInput{
		Profile: &model.AnalyzerPolicy{CoverageReport: "out/p.cov", Coverage: model.CoverProfile{CoverPkg: true, CoverMode: "atomic"}}, Profiled: "surface",
	})
	if err != nil || profiled.CoverageReport != "out/p.cov" || profiled.Coverage != (model.CoverProfile{CoverPkg: true, CoverMode: "atomic"}) {
		t.Fatalf("profiled = %+v, %v, want the profile's report and profile", profiled, err)
	}
}

func TestLinesPrintTheScopeInForce(t *testing.T) {
	t.Parallel()
	s := propertiesScope(t, "sonar.sources=cmd,internal\nsonar.exclusions=**/*_test.go,integration/**\nsonar.cpd.exclusions=**/gen/**\nsonar.inclusions=${X}\n")
	origin := " (sonar-project.properties)"
	want := []string{
		"surface.scope.source = properties" + origin,
		"surface.scope.origin = sonar-project.properties" + origin,
		"surface.scope.sources = [cmd, internal]" + origin,
		"surface.scope.exclusions = [**/*_test.go, integration/**]" + origin,
		"surface.scope.cpd_exclusions = [**/gen/**]" + origin,
		"surface.scope.coverage_report = profile.cov" + origin,
		"surface.scope.coverage_profile = coverpkg false, covermode set" + origin,
		"surface.scope.unmeasured = sonar.inclusions: a ${...} placeholder golaw does not resolve" + origin,
	}
	if got := s.Lines(); !slices.Equal(got, want) {
		t.Errorf("Lines =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	none, err := surface.Resolve(surface.ScopeInput{Policy: policyOf(model.FromAuto)})
	if err != nil {
		t.Fatal(err)
	}
	want = []string{
		"surface.scope.source = none (computed)",
		"surface.scope.origin = none (computed)",
		"surface.scope.coverage_report = profile.cov (computed)",
		"surface.scope.coverage_profile = coverpkg false, covermode set (computed)",
	}
	if got := none.Lines(); !slices.Equal(got, want) {
		t.Errorf("Lines of none =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/profiletest"
)

// corePolicy is a golaw.yml pinned at the test version and naming nothing
// else, so the surface lines come from the tree alone; the file itself is
// YAML, so every languages line names yaml.
const corePolicy = "golaw: 1\nkit: {version: " + testVersion + "}\n"

// pipelineProperties is the shape of a Go pipeline's scanner properties.
const pipelineProperties = "sonar.projectKey=hub\nsonar.exclusions=**/*_test.go,integration/**,**/testdata/**\nsonar.go.coverage.reportPaths=profile.cov\n"

// surfaceRepository commits a golaw.yml and the files of a test into a
// fresh repository and answers its root.
func surfaceRepository(t *testing.T, policyText string, files map[string]string) string {
	t.Helper()
	root := repository(t, false)
	files[policy.FileName] = policyText
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "build: the surface fixture")
	return root
}

// explainApp is a test app whose explain seam and binding are the
// production ones over a temporary home.
func explainApp(t *testing.T) (testApp, binding) {
	t.Helper()
	ta := newTestApp(t)
	b, _ := testBinding(t)
	ta.explain = b.explain
	ta.bind = func() binding { return b }
	return ta, b
}

// explained runs config explain over root and answers stdout.
func explained(t *testing.T, ta testApp, root string) string {
	t.Helper()
	if got := ta.run(t.Context(), []string{"config", "explain", root}); got != exitClean {
		t.Fatalf("exit = %d, stderr = %q", got, ta.stderr.String())
	}
	return ta.stdout.String()
}

// mustHold fails the test for each line the output lacks.
func mustHold(t *testing.T, out string, lines ...string) {
	t.Helper()
	for _, line := range lines {
		if !strings.Contains(out, line+"\n") {
			t.Errorf("config explain lacks %q:\n%s", line, out)
		}
	}
}

func TestConfigExplainPrintsThePropertiesScopeAndTheLanguages(t *testing.T) {
	t.Parallel()
	ta, _ := explainApp(t)
	root := surfaceRepository(t, corePolicy, map[string]string{
		"sonar-project.properties": pipelineProperties,
		"views/panel.html":         "<script>\nrun();\n</script>\n",
		"Dockerfile":               "FROM scratch\n",
		"scripts/smoke.sh":         "#!/bin/sh\n",
	})
	out := explained(t, ta, root)
	mustHold(t, out,
		"surface.scope.source = properties (sonar-project.properties)",
		"surface.scope.origin = sonar-project.properties (sonar-project.properties)",
		"surface.scope.exclusions = [**/*_test.go, integration/**, **/testdata/**] (sonar-project.properties)",
		"surface.scope.coverage_report = profile.cov (sonar-project.properties)",
		"surface.languages = [html, dockerfile, shell, yaml] (computed)",
	)
	if !strings.Contains(out, "kit.version") || strings.Contains(out, "surface.shape") {
		t.Errorf("the policy lines come first and no shape line is printed:\n%s", out)
	}
	if policyAt, scopeAt := strings.Index(out, "kit.version"), strings.Index(out, "surface.scope.source"); policyAt > scopeAt {
		t.Errorf("the surface lines follow the policy lines:\n%s", out)
	}
}

func TestConfigExplainPrefersThePinnedProfilesScope(t *testing.T) {
	t.Parallel()
	ta, b := explainApp(t)
	dirs, err := b.dirs()
	if err != nil {
		t.Fatal(err)
	}
	profiletest.SurfaceInstall(t, dirs.Config)
	ref := profiletest.SurfaceRef(t)
	pinned := corePolicy + fmt.Sprintf("profile: {name: %s, source: %q, version: %q, digest: %q}\n", ref.Name, ref.Source, ref.Version, ref.Digest)
	root := surfaceRepository(t, pinned, map[string]string{"sonar-project.properties": pipelineProperties})
	mustHold(t, explained(t, ta, root),
		"surface.scope.source = profile (surface/analyzer.yml)",
		"surface.scope.exclusions = [**/*_test.go, integration/**, **/gen/**, **/testdata/**, examples/*] (surface/analyzer.yml)",
		"surface.scope.coverage_profile = coverpkg false, covermode set (surface/analyzer.yml)",
		"surface.languages = [yaml] (computed)",
	)
}

func TestConfigExplainNamesTheShapeAndTheFilesItCannotJudge(t *testing.T) {
	t.Parallel()
	ta, _ := explainApp(t)
	root := surfaceRepository(t, corePolicy, map[string]string{
		"package.json":     `{"scripts":{"build":"tsc"}}`,
		"web/app.ts":       "export const a = 1;\n",
		"infra/broken.yml": "AWSTemplateFormatVersion: [\n",
	})
	out := explained(t, ta, root)
	mustHold(t, out,
		"surface.scope.source = none (computed)",
		"surface.languages = [typescript, yaml] (computed)",
		"surface.shape = package.json declares a build script: a Node or TypeScript project is outside M6, which judges the files it reads (computed)",
	)
	if !strings.Contains(out, "surface.unmeasured = infra/broken.yml: cloudformation template unreadable: ") {
		t.Errorf("config explain names the broken template:\n%s", out)
	}
}

func TestConfigExplainRefusesAPinnedScopeSourceThatIsAbsent(t *testing.T) {
	t.Parallel()
	ta, _ := explainApp(t)
	root := surfaceRepository(t, corePolicy+"surface:\n  analyzer:\n    from: properties\n", map[string]string{"views/a.js": "a()\n"})
	if got := ta.run(t.Context(), []string{"config", "explain", root}); got != exitFailure {
		t.Fatalf("exit = %d, stdout = %q", got, ta.stdout.String())
	}
	want := "analyzer scope source absent: surface.analyzer.from is properties and the repository root holds no sonar-project.properties"
	if got := ta.stderr.String(); !strings.HasPrefix(got, "golaw: config explain: resolve the analyzer scope of ") || !strings.Contains(got, want) {
		t.Fatalf("stderr = %q, want the pinned source named", got)
	}
	if ta.stdout.Len() != 0 {
		t.Fatalf("a refused explanation prints nothing: %q", ta.stdout.String())
	}
}

func TestSurfaceExplainReportsAFailedLoadAndSnapshot(t *testing.T) {
	t.Parallel()
	_, b := explainApp(t)
	outside := t.TempDir()
	if _, err := b.surfaceExplain(t.Context(), outside, now()); err == nil {
		t.Fatal("a directory outside git explained its surface")
	}
}

func TestConfigExplainReportsAFailedWrite(t *testing.T) {
	t.Parallel()
	root := surfaceRepository(t, corePolicy, map[string]string{"views/a.js": "a()\n"})
	for name, tc := range map[string]struct {
		writes int
		want   string
	}{
		"the policy lines": {writes: 0, want: "golaw: config explain: write output: boom\n"},
		"a surface line":   {writes: 1, want: "golaw: config explain: write output: boom\n"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ta, _ := explainApp(t)
			ta.app.stdout = &doctorFailWriter{remaining: tc.writes}
			if got := ta.run(t.Context(), []string{"config", "explain", root}); got != exitFailure || ta.stderr.String() != tc.want {
				t.Fatalf("exit = %d, stderr = %q, want %q", got, ta.stderr.String(), tc.want)
			}
		})
	}
}

package cicheck_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	checksurface "github.com/cristianargotti/golaw/internal/checks/surface"
	"github.com/cristianargotti/golaw/internal/lint"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

const (
	m6SurfacePath   = "docs/surface.md"
	m6RuleMapHeader = "| Sonar key | Tool | Local rule | Where | Match | Set |"
	m6ToolHeader    = "| Tool | Version | Distribution | Languages | Hook |"
	m6NativeHeader  = "| Check | Sonar key | Answer |"
)

// m6TableRows answers the rows of the table under header: the lines after
// its separator up to the first line that is not a row.
func m6TableRows(text, header string) []string {
	_, rest, found := strings.Cut(text, "\n"+header+"\n")
	if !found {
		return nil
	}
	lines := strings.Split(rest, "\n")
	var rows []string
	for _, line := range lines[1:] {
		if !strings.HasPrefix(line, "| ") {
			break
		}
		rows = append(rows, line)
	}
	return rows
}

func m6RuleMapWant() []string {
	rows := lint.Rows()
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, fmt.Sprintf("| %s | %s | %s | %s | %s | %s |", r.Sonar, r.Tool, r.Rule, strings.Join(r.Where, ", "), r.Match, r.Set))
	}
	return out
}

func m6ToolWant(t *testing.T) []string {
	t.Helper()
	tools, err := toolchain.Table()
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(tools))
	for i := range tools {
		tool := &tools[i]
		languages := make([]string, 0, len(tool.Languages))
		for _, language := range tool.Languages {
			languages = append(languages, string(language))
		}
		hook := "no"
		if tool.Hook {
			hook = "yes"
		}
		out = append(out, fmt.Sprintf("| %s | %s | %s | %s | %s |", tool.Name, tool.Version, tool.Distribution, strings.Join(languages, ", "), hook))
	}
	return out
}

// m6SurfaceStatements are the sentences of the page a reader relies on.
func m6SurfaceStatements() []string {
	return []string{
		"# The non-Go surface",
		"Every check on the developer's machine is optional. The pipeline or remote CI is the only gate.",
		"golaw never calls a Sonar server, a scanner or its API, never reads a token for it and never imports its results",
		"It describes the pipeline and never relaxes a core law",
		"because a platform pipeline may delete the repository's own sonar-project.properties before it scans",
		"a company's pipeline exclusions live in its private profile, never in this kit",
		"A missing tool is a doctor line and a notice, never a denial.",
		"killed at 1.5 s, and the external work of one event stops at 3 s",
		"under 1 s at the 95th percentile of 20 warm runs",
		"The kit's own make ci runs lint-surface and coverage-new over its own tree",
		"docker:S7020", "docker:S6470", "docker:S6504", "docker:S6505", "docker:S2612",
		"python:S1313", "python:S5852", "python:S1192", "cloudformation:S6304", "cloudformation:S6302",
		"Web:S5254", "Web:S1077",
		"export the code a test needs",
		"golaw lint reports each finding ESLint suppressed as a note",
	}
}

func TestM6SurfaceGuideRendersItsSources(t *testing.T) {
	t.Parallel()
	text := string(kitBytes(t, m6SurfacePath))
	for _, tc := range []struct {
		name, header string
		want         []string
	}{
		{"rule map", m6RuleMapHeader, m6RuleMapWant()},
		{"toolchain", m6ToolHeader, m6ToolWant(t)},
	} {
		if len(tc.want) == 0 {
			t.Fatalf("the %s source is empty", tc.name)
		}
		if got := m6TableRows(text, tc.header); !slices.Equal(got, tc.want) {
			t.Errorf("the %s table of %s differs from its source; write exactly these rows under its header:\n%s", tc.name, m6SurfacePath, strings.Join(tc.want, "\n"))
		}
		dropped := strings.Replace(text, tc.want[0]+"\n", "", 1)
		if slices.Equal(m6TableRows(dropped, tc.header), tc.want) {
			t.Errorf("red: a dropped %s row went unseen", tc.name)
		}
	}
}

func TestM6SurfaceGuideNamesEveryNativeCheck(t *testing.T) {
	t.Parallel()
	rows := m6TableRows(string(kitBytes(t, m6SurfacePath)), m6NativeHeader)
	checks := checksurface.Checks()
	if len(rows) != len(checks) {
		t.Fatalf("the native table holds %d rows for %d checks", len(rows), len(checks))
	}
	for _, check := range checks {
		id := check.Spec().ID
		if !slices.ContainsFunc(rows, func(row string) bool { return strings.HasPrefix(row, "| "+id+" |") }) {
			t.Errorf("the native table has no row for %s", id)
		}
	}
}

func TestM6SurfaceGuideStatesItsBoundaries(t *testing.T) {
	t.Parallel()
	text := string(kitBytes(t, m6SurfacePath))
	statements := m6SurfaceStatements()
	forbidden := []string{"golaw calls the Sonar", "golaw installs Node", "the hook downloads", "ruff --isolated"}
	if !m4TextOK(text, statements, forbidden) {
		for _, statement := range statements {
			if !strings.Contains(text, statement) {
				t.Errorf("%s lacks %q", m6SurfacePath, statement)
			}
		}
		t.Fatalf("%s states a forbidden claim or lacks a statement", m6SurfacePath)
	}
	for _, red := range []string{statements[1], statements[9], "docker:S6470"} {
		if m4TextOK(strings.Replace(text, red, "", 1), statements, forbidden) {
			t.Errorf("red: the page without %q passed", red)
		}
	}
	if m4TextOK(text+"\n"+forbidden[0]+" server.\n", statements, forbidden) {
		t.Error("red: a claim that golaw calls the analyzer passed")
	}
}

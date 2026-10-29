package surface_test

import (
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/surface"
	"github.com/cristianargotti/golaw/internal/model"
)

const props = "sonar-project.properties"

// widen judges an agent's write of the properties file over a base that
// holds base, or none when base is empty.
func widen(t *testing.T, base, candidate string) model.Result {
	t.Helper()
	c := surface.AnalyzerWidening()
	files := map[string]string{}
	if base != "" {
		files[props] = base
	}
	return run(t, c, writeInput(c, files, map[string]string{props: candidate}))
}

func TestAnalyzerWideningFindsWhatAnAgentHides(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, base, candidate string
		count                 int
	}{
		{"an exclusion added", "sonar.exclusions=**/*_test.go\n", "sonar.exclusions=**/*_test.go,src/legacy/**\n", 1},
		{"a coverage exclusion in a new file", "", "sonar.coverage.exclusions=cmd/**\n", 1},
		{"a duplication exclusion added", "sonar.projectKey=hub\n", "sonar.projectKey=hub\nsonar.cpd.exclusions=gen/**\n", 1},
		{"a test inclusion added", "sonar.test.inclusions=**/*_test.go\n", "sonar.test.inclusions=**/*_test.go,internal/**\n", 1},
		{"sources narrowed from the whole tree", "sonar.projectKey=hub\n", "sonar.projectKey=hub\nsonar.sources=cmd\n", 1},
		{"sources narrowed by an entry", "sonar.sources=cmd,internal\n", "sonar.sources=cmd\n", 1},
		{"an issue ignore key added", "sonar.projectKey=hub\n", "sonar.projectKey=hub\nsonar.issue.ignore.multicriteria=e1\n", 1},
		{"an exclusion on a continuation line", "sonar.exclusions=a/**\n", "sonar.exclusions=a/**,\\\n  b/**\n", 1},
		{"two exclusions added", "", "sonar.exclusions=a/**,b/**\n", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			res := widen(t, tt.base, tt.candidate)
			if len(res.Findings) != tt.count {
				t.Fatalf("findings = %+v, want %d", res.Findings, tt.count)
			}
			for _, f := range res.Findings {
				if f.Path != props || f.Line != 0 || f.Code != "analyzer-widening" || f.Severity != model.SeverityDeny {
					t.Errorf("finding = %+v", f)
				}
			}
		})
	}
}

func TestAnalyzerWideningLeavesTheNearMissesAlone(t *testing.T) {
	t.Parallel()
	hub := "sonar.projectKey=hub\nsonar.exclusions=**/*_test.go,integration/**,**/testdata/**\nsonar.go.coverage.reportPaths=profile.cov\n"
	tests := []struct{ name, base, candidate string }{
		{"an exclusion removed", "sonar.exclusions=a/**,b/**\n", "sonar.exclusions=a/**\n"},
		{"entries reordered", "sonar.exclusions=a/**,b/**\n", "sonar.exclusions=b/**, a/**\n"},
		{"tests named", hub, hub + "sonar.tests=test\n"},
		{"sources widened", "sonar.sources=cmd\n", "sonar.sources=cmd,internal\n"},
		{"the project key changed", hub, "sonar.projectKey=panel" + hub[len("sonar.projectKey=hub"):]},
		{"the hub's file unchanged", hub, hub},
		{"a commented exclusion", hub, hub + "# sonar.exclusions=src/**\n"},
		{"a placeholder entry", "", "sonar.exclusions=${EXTRA}\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			expectFound(t, widen(t, tt.base, tt.candidate))
		})
	}
}

func TestAnalyzerWideningNamesWhatIsHidden(t *testing.T) {
	t.Parallel()
	res := widen(t, "sonar.sources=cmd,internal\n", "sonar.sources=cmd\nsonar.exclusions=src/legacy/**\nsonar.issue.ignore.multicriteria=e1\n")
	want := []model.Finding{
		{Path: props, Code: "analyzer-widening", Severity: model.SeverityDeny, Message: "adds src/legacy/** to sonar.exclusions: an exclusion hides code from the pipeline's gate; fix the findings, or ask the person to change the scope."},
		{Path: props, Code: "analyzer-widening", Severity: model.SeverityDeny, Message: "narrows sonar.sources to cmd: a narrower scope hides code from the pipeline's gate; fix the findings, or ask the person to change the scope."},
		{Path: props, Code: "analyzer-widening", Severity: model.SeverityDeny, Message: "adds sonar.issue.ignore.multicriteria, which hides findings from the pipeline's gate; fix the findings, or ask the person to change the scope."},
	}
	if !slices.EqualFunc(res.Findings, want, sameFinding) {
		t.Fatalf("findings = %+v\nwant %+v", res.Findings, want)
	}
}

func TestAnalyzerWideningJudgesAgentWritesOnly(t *testing.T) {
	t.Parallel()
	c := surface.AnalyzerWidening()
	if c.Spec().Scope != model.ScopeEvent {
		t.Fatalf("scope = %s, want event", c.Spec().Scope)
	}
	files := map[string]string{props: "sonar.exclusions=src/**\n"}
	for name, in := range map[string]model.Input{"a scan": scanInput(c, files), "a post event": postInput(c, files)} {
		res, err := c.Run(t.Context(), in)
		if err != nil || res.Status.Kind != model.StatusNotApplicable || len(res.Findings) != 0 {
			t.Errorf("%s = %+v, %v, want not applicable", name, res, err)
		}
	}
	nested := map[string]string{"tools/" + props: "sonar.exclusions=src/**\n"}
	expectFound(t, run(t, c, writeInput(c, nil, nested)))
	edit := writeInput(c, files, map[string]string{props: "sonar.exclusions=src/**,lib/**\n"})
	edit.Snapshot = tree(map[string]string{props: "sonar.exclusions=src/**,lib/**\n"})
	expectFound(t, run(t, c, edit))
}

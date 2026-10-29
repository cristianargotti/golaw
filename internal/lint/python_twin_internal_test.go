package lint

import (
	"maps"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/surfacefix"
)

// pythonKey names a mapped finding by its path and its code.
type pythonKey struct {
	path, code string
}

//nolint:paralleltest // the live runs resolve the tools through the test environment
func TestPythonDriversOnTheTwins(t *testing.T) {
	tests := map[string]struct {
		twin surfacefix.Twin
		want map[pythonKey]int
	}{
		"the red twin yields the ruff expectations":               {twin: surfacefix.Red, want: pythonTwinExpectations()},
		"the corrected twin yields no mapped finding (near miss)": {twin: surfacefix.Corrected, want: map[pythonKey]int{}},
	}
	for name, tc := range tests {
		for _, set := range []string{model.SurfaceParity, model.SurfaceStrict} {
			t.Run(name+" under "+set, func(t *testing.T) {
				if got := pythonMapped(pythonTwinIssues(t, tc.twin, set)); !maps.Equal(got, tc.want) {
					t.Errorf("mapped findings = %v, want %v", got, tc.want)
				}
			})
		}
	}
}

func TestPythonTwinExpectationsNameTheHubClasses(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		key  pythonKey
		want int
	}{
		"the literal password of the test (python:S2068)":    {key: pythonKey{"test/deploy/test_install.py", "s105"}, want: 1},
		"the dict comprehension of None (python:S7519)":      {key: pythonKey{"test/deploy/test_install.py", "c420"}, want: 1},
		"the install module holds none (near miss)":          {key: pythonKey{"deploy/install.py", "s105"}, want: 0},
		"the password in the URL is native only (near miss)": {key: pythonKey{"test/deploy/test_install.py", "s106"}, want: 0},
	}
	got := pythonTwinExpectations()
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got[tc.key] != tc.want {
				t.Errorf("expectation %v = %d, want %d", tc.key, got[tc.key], tc.want)
			}
		})
	}
}

// pythonTwinExpectations keeps the expectations of the red twin that name
// a Python tool, keyed as pythonMapped keys the findings.
func pythonTwinExpectations() map[pythonKey]int {
	want := make(map[pythonKey]int)
	for _, e := range surfacefix.Expectations() {
		if e.Check == "lint."+pythonRuff || e.Check == "lint."+pythonPyright {
			want[pythonKey{e.Path, e.Code}] += e.Count
		}
	}
	return want
}

// pythonTwinIssues writes a twin into a fresh root and runs ruff check and
// pyright over its Python files under the set.
func pythonTwinIssues(t *testing.T, twin surfacefix.Twin, set string) []Issue {
	t.Helper()
	root := pythonRoot(t)
	if err := surfacefix.Write(root, twin); err != nil {
		t.Fatalf("write the twin: %v", err)
	}
	files := pythonFiles(t, root)
	formats := []Format{FormatRuff, FormatPyright}
	issues := make([]Issue, 0, len(formats))
	for _, format := range formats {
		issues = append(issues, runPythonDriver(t, pythonDriver(t, format), set, root, files)...)
	}
	return issues
}

// pythonMapped counts the issues whose rule a Python row maps; an unmapped
// strict finding is the strict set's business, not the twin's.
func pythonMapped(issues []Issue) map[pythonKey]int {
	mapped := make(map[string]bool)
	for _, r := range pythonRows() {
		mapped[r.Tool+"/"+r.Rule] = true
	}
	got := make(map[pythonKey]int)
	for _, i := range issues {
		if mapped[i.Tool+"/"+i.Rule] {
			got[pythonKey{i.Path, Code(i.Rule)}]++
		}
	}
	return got
}

// TestTheCorrectedTwinIsCleanUnderTheStrictPythonSet: under strict the
// corrected twin yields no finding at all, mapped or not, so the set init
// selects never reds a tree the kit itself calls clean.
//
//nolint:paralleltest // the live runs resolve the tools through the test environment
func TestTheCorrectedTwinIsCleanUnderTheStrictPythonSet(t *testing.T) {
	for _, issue := range pythonTwinIssues(t, surfacefix.Corrected, model.SurfaceStrict) {
		t.Errorf("%s:%d %s/%s %s", issue.Path, issue.Line, issue.Tool, issue.Rule, issue.Message)
	}
}

package surface_test

import (
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/surface"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestCertainChecksDenyOnlyWhatTheCandidateIntroduces(t *testing.T) {
	t.Parallel()
	legacy := map[string]string{"app.js": "eval(one);\n"}
	tests := []struct {
		name      string
		base      map[string]string
		candidate map[string]string
		want      []string
	}{
		{"a new file", nil, map[string]string{"app.js": "eval(one);\n"}, []string{"app.js:1 code-eval"}},
		{"a legacy call kept", legacy, map[string]string{"app.js": "// first\neval(one);\n"}, nil},
		{"a second call added", legacy, map[string]string{"app.js": "eval(one);\neval(two);\n"}, []string{"app.js:2 code-eval"}},
		{"a legacy call moved", legacy, map[string]string{"app.js": "const a = 1;\n\neval(one);\n"}, nil},
		{"a new file beside a legacy one", legacy, map[string]string{"b.js": "eval(one);\n"}, []string{"b.js:1 code-eval"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := surface.CodeEval()
			res := run(t, c, writeInput(c, tt.base, tt.candidate))
			expectFound(t, res, tt.want...)
			for _, f := range res.Findings {
				if f.Severity != model.SeverityDeny {
					t.Errorf("%s:%d severity = %s, want deny", f.Path, f.Line, f.Severity)
				}
			}
		})
	}
}

func TestAPartialCandidateIsLeftUnread(t *testing.T) {
	t.Parallel()
	c := surface.CodeEval()
	res := run(t, c, writeInput(c, map[string]string{"app.js": "run();\n"}, map[string]string{"app.js": "eval(two);\n"}, "app.js"))
	expectFound(t, res)
	if !slices.Equal(res.Unread, []string{"app.js"}) {
		t.Fatalf("unread = %q, want app.js", res.Unread)
	}
}

func TestAPostEventFeedsCertainFindingsBackAsNotes(t *testing.T) {
	t.Parallel()
	c := surface.CodeEval()
	res := run(t, c, postInput(c, map[string]string{"app.js": "eval(one);\n"}))
	expectFound(t, res, "app.js:1 code-eval")
	if res.Findings[0].Severity != model.SeverityNote {
		t.Fatalf("severity = %s, want note: a post event has no base to tell introduced from legacy", res.Findings[0].Severity)
	}
}

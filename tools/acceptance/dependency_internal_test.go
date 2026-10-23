package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	scanreport "github.com/cristianargotti/golaw/internal/report"
)

// replaceDocument is the scan answer of a replace repository: the status of
// the dependency check and its reason, as golaw scan --json prints them.
func replaceDocument(t *testing.T, status, reason string) string {
	t.Helper()
	data, err := json.Marshal(scanreport.Document{Schema: scanreport.Schema, Checks: []scanreport.Status{
		{ID: dependencyCheck, Rule: "rules/4.8@1", Status: status, Reason: reason},
	}})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// dependencyAnswers answers the scan of the two replace repositories as the
// kit does, and sends every other command on.
func dependencyAnswers(next executor) executor {
	return func(ctx context.Context, c command) result {
		if len(c.args) == 3 && c.args[0] == commandScan && c.args[1] == "--json" {
			switch filepath.Base(c.dir) {
			case caseInternalReplace:
				return result{stdout: `{"schema":1,"checks":[{"id":"policy.dependency-records","rule":"rules/4.8@1","status":"measured"}]}`}
			case caseExternalReplace:
				return result{code: 2, stdout: `{"schema":1,"checks":[{"id":"policy.dependency-records","rule":"rules/4.8@1",` +
					`"status":"unmeasured","reason":"go.mod:5: replace example.test/library => ../external-library: outside the repository"}]}`}
			}
		}
		return next(ctx, c)
	}
}

func TestJudgeReplace(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, row, status, reason string
		code                      int
		fail                      bool
	}{
		{"near internal followed", caseInternalReplace, "measured", "", 0, false},
		{"red internal left unmeasured", caseInternalReplace, "unmeasured", "replace", 2, true},
		{"near external named", caseExternalReplace, "unmeasured", "go.mod:5: replace example.test/library => ../external-library", 2, false},
		{"red external clean", caseExternalReplace, "measured", "", 0, true},
		{"red external without the directive", caseExternalReplace, "unmeasured", "outside the declared subset", 2, true},
	} {
		got := result{code: tc.code, stdout: replaceDocument(t, tc.status, tc.reason)}
		if err := judgeReplace(tc.row, "../external-library", got); (err != nil) != tc.fail {
			t.Fatalf("%s: judge = %v", tc.name, err)
		}
	}
	if err := judgeReplace(caseInternalReplace, "./library", result{stdout: "not json"}); err == nil {
		t.Fatal("a broken document must fail")
	}
	if err := judgeReplace(caseInternalReplace, "./library", result{stdout: `{"schema":1,"checks":[]}`}); err == nil {
		t.Fatal("a scan without the dependency check must fail")
	}
}

func TestDependencyRowsBuildBothReplaceRepositories(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	s.execute = dependencyAnswers(newM2Script(t, s, func(_ context.Context, c command) (m2Reply, bool) {
		if c.program == s.binary && len(c.args) > 0 && c.args[0] == commandInit {
			return m2Reply{got: m2Init(t, s, c)}, true
		}
		return m2Reply{}, false
	}).Execute)
	rows := s.dependencyRows(t.Context())
	if len(rows) != 2 || slices.ContainsFunc(rows, func(item row) bool { return item.status != statusPass || !item.required }) {
		t.Fatalf("rows = %+v", rows)
	}
	module, err := os.ReadFile(filepath.Join(s.base, dirRepos, caseExternalReplace, moduleFile))
	if err != nil || !strings.Contains(string(module), "replace example.test/library => ../external-library\n") {
		t.Fatalf("external go.mod = %q, %v", module, err)
	}
	if _, err := os.Stat(filepath.Join(s.base, dirRepos, caseInternalReplace, "library", moduleFile)); err != nil {
		t.Fatalf("the internal library is missing: %v", err)
	}
}

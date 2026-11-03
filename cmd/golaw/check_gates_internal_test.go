package main

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/check"
	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/runnertest"
)

// gatePlan is a plan over the fixture's base commit with the scope and
// the capabilities a row names, every tool missing until the row sets one.
func gatePlan(t *testing.T, f checkFixture, capabilities string, scope check.Scope) *checkPlan {
	t.Helper()
	policy, err := f.ta.bind().compile([]byte(checkPolicy(capabilities)), now())
	if err != nil {
		t.Fatal(err)
	}
	return &checkPlan{
		a: f.ta.app, root: f.repo.Root, policy: policy, scope: scope, module: checkModule,
		affected: runner.AffectedSet{Patterns: []string{"."}}, env: gittest.Isolated(), temp: t.TempDir(),
	}
}

func fake(t *testing.T, dir, name string, b runnertest.Behavior) string {
	t.Helper()
	runnertest.Install(t, dir, name, runnertest.Behavior{})
	return runnertest.Install(t, dir, name, b)
}

func only(t *testing.T, wait check.Wait) check.Result {
	t.Helper()
	results := wait()
	if len(results) != 1 {
		t.Fatalf("results = %+v", results)
	}
	return results[0]
}

func TestMigrationsGate(t *testing.T) {
	t.Parallel()
	changed := check.Scope{Base: "base", Paths: []string{"db/migrations/001_init.sql", "db/migrations/notes.md"}}
	for _, tc := range []struct {
		name         string
		capabilities string
		scope        check.Scope
		squawk       *runnertest.Behavior
		status       check.Status
		want         string
	}{
		{"near-the-db-capability-off", "go", changed, nil, check.Skipped, "the db capability is off"},
		{"near-no-migration-changed", "go, db", check.Scope{Base: "base", Paths: []string{"fx.go"}}, nil, check.Skipped, "no migration changed"},
		{"near-squawk-missing", "go, db", changed, nil, check.Skipped, "squawk is not on PATH"},
		{"red-squawk-finds-a-defect", "go, db", changed, &runnertest.Behavior{Stdout: "db/migrations/001_init.sql:1:1: warning: prefer-robust-stmts\n", Code: 1}, check.Red, "prefer-robust-stmts"},
		{"near-squawk-is-clean", "go, db", check.Scope{Whole: true}, &runnertest.Behavior{}, check.Green, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newCheckFixture(t, "go")
			f.repo.Write("db/migrations/001_init.sql", "create table t (id int);\n")
			f.repo.Write("db/migrations/notes.md", "notes\n")
			p := gatePlan(t, f, tc.capabilities, tc.scope)
			if tc.squawk != nil {
				p.tools.squawk = fake(t, f.tools, squawkTool, *tc.squawk)
			}
			got := only(t, p.migrationsGate(t.Context()))
			if got.Status != tc.status || !strings.Contains(got.Reason+strings.Join(got.Failures, "\n"), tc.want) {
				t.Fatalf("result = %+v", got)
			}
			if tc.squawk != nil {
				calls := runnertest.Calls(t, f.tools, squawkTool)
				if want := filepath.FromSlash("db/migrations/001_init.sql"); !slices.Equal(calls[len(calls)-1], []string{want}) {
					t.Errorf("squawk calls = %q", calls)
				}
			}
		})
	}
}

func TestFormatAndLintGates(t *testing.T) {
	t.Parallel()
	diff := "--- fx.go.orig\n+++ fx.go\n@@ -1 +1 @@\n-a\n+b\n"
	for _, tc := range []struct {
		name   string
		scope  check.Scope
		linter runnertest.Behavior
		gate   func(*checkPlan, context.Context) check.Wait
		status check.Status
		want   string
		call   string
	}{
		{"red-an-unformatted-file", check.Scope{Base: "base", Paths: []string{"fx.go", "gone.go"}}, runnertest.Behavior{Stdout: diff, Code: 1}, (*checkPlan).formatGate, check.Red, "fx.go: not formatted", "fmt --diff fx.go"},
		{"red-a-formatter-that-fails-silently", check.Scope{Whole: true}, runnertest.Behavior{Stderr: "level=error config\n", Code: 3}, (*checkPlan).formatGate, check.Red, "level=error config", "fmt --diff ./..."},
		{"near-no-go-file-changed", check.Scope{Base: "base", Paths: []string{"README.md"}}, runnertest.Behavior{}, (*checkPlan).formatGate, check.Skipped, "no Go file changed", ""},
		{"red-a-changed-configuration-lints-the-module", check.Scope{Base: "base", Paths: []string{".golangci.yml"}}, runnertest.Behavior{Stdout: "a.go:1:1: x (revive)\n", Code: 1}, (*checkPlan).lintGate, check.Red, "a.go:1:1: x (revive)", "--new-from-merge-base=base ./..."},
		{"near-the-whole-tree-lints-without-a-period", check.Scope{Whole: true}, runnertest.Behavior{}, (*checkPlan).lintGate, check.Green, "", "--show-stats=false ."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newCheckFixture(t, "go")
			p := gatePlan(t, f, "go", tc.scope)
			p.tools.lint = fake(t, f.tools, linterTool, tc.linter)
			got := only(t, tc.gate(p, t.Context()))
			if got.Status != tc.status || !strings.Contains(got.Reason+strings.Join(got.Failures, "\n"), tc.want) {
				t.Fatalf("result = %+v", got)
			}
			calls := runnertest.Calls(t, f.tools, linterTool)
			if tc.call != "" && (len(calls) == 0 || !strings.HasSuffix(strings.Join(calls[len(calls)-1], " "), tc.call)) {
				t.Errorf("calls = %q, want one ending %q", calls, tc.call)
			}
		})
	}
}

// The Go gates skip, each with its reason, when they do not apply, and a
// tool that cannot start is red, never a pass.
func TestGoGatesSkipOrFail(t *testing.T) {
	t.Parallel()
	f := newCheckFixture(t, "go")
	scope := check.Scope{Base: "base", Paths: []string{"fx.go"}}
	off := gatePlan(t, f, "", scope)
	moduleless := gatePlan(t, f, "go", scope)
	moduleless.module = ""
	unchanged := gatePlan(t, f, "go", scope)
	unchanged.affected = runner.AffectedSet{}
	broken := gatePlan(t, f, "go", scope)
	broken.tools = checkTools{golang: filepath.Join(f.tools, "absent-go")}
	for _, tc := range []struct {
		name   string
		got    []check.Result
		status check.Status
		want   string
	}{
		{"near-the-go-capability-off", off.vetGate(t.Context())(), check.Skipped, "the go capability is off"},
		{"near-no-go-mod", moduleless.testGate(t.Context())(), check.Skipped, "the repository root holds no go.mod"},
		{"near-no-package-changed", unchanged.lintGate(t.Context())(), check.Skipped, "no Go package changed"},
		{"near-go-missing", gatePlan(t, f, "go", scope).vetGate(t.Context())(), check.Skipped, "go is not on PATH"},
		{"near-go-missing-for-test", gatePlan(t, f, "go", scope).testGate(t.Context())(), check.Skipped, "go is not on PATH"},
		{"red-vet-cannot-start", broken.vetGate(t.Context())(), check.Red, "could not run absent-go"},
		{"red-test-cannot-start", broken.testGate(t.Context())(), check.Red, "could not run go test"},
	} {
		if tc.got[0].Status != tc.status || !strings.Contains(tc.got[0].Reason+strings.Join(tc.got[0].Failures, "\n"), tc.want) {
			t.Errorf("%s: result = %+v", tc.name, tc.got)
		}
	}
}

func TestScanGateKeepsTheFindingsOfTheChange(t *testing.T) {
	t.Parallel()
	f := newCheckFixture(t, "go")
	whole := only(t, gatePlan(t, f, "go", check.Scope{Whole: true}).scanGate(t.Context()))
	if whole.Status != check.Red || !slices.ContainsFunc(whole.Failures, func(line string) bool { return strings.HasPrefix(line, ".golangci.yml policy.golangci: ") }) {
		t.Errorf("red whole tree = %+v", whole)
	}
	if near := only(t, gatePlan(t, f, "go", check.Scope{Base: "base", Paths: []string{"fx.go"}}).scanGate(t.Context())); near.Status != check.Green {
		t.Errorf("near change = %+v", near)
	}
	f.ta.snapshot = func(context.Context, string) (model.Snapshot, error) { return nil, errors.ErrUnsupported }
	if failed := only(t, gatePlan(t, f, "go", check.Scope{Whole: true}).scanGate(t.Context())); failed.Status != check.Red {
		t.Errorf("unreadable worktree = %+v", failed)
	}
	for _, tc := range []struct {
		finding model.Finding
		want    string
	}{
		{model.Finding{Check: "c", Path: "a.go", Line: 3, Message: "m"}, "a.go:3 c: m"},
		{model.Finding{Check: "c", Path: "a.go", Message: "m"}, "a.go c: m"},
		{model.Finding{Check: "c", Message: "m"}, "c: m"},
	} {
		if got := findingLine(tc.finding); got != tc.want {
			t.Errorf("findingLine = %q, want %q", got, tc.want)
		}
	}
}

func TestCoverageAndPlanEdges(t *testing.T) {
	t.Parallel()
	f := newCheckFixture(t, "go")
	if got := gatePlan(t, f, "go", check.Scope{Whole: true}).coverage(t.Context(), "p.cov"); got.Status != check.Skipped {
		t.Errorf("whole tree coverage = %+v", got)
	}
	if got := gatePlan(t, f, "go", check.Scope{Base: "nowhere"}).coverage(t.Context(), "p.cov"); got.Status != check.Red {
		t.Errorf("unknown base coverage = %+v", got)
	}
	if got := gatePlan(t, newCheckFixture(t, ""), "", check.Scope{Base: "base"}).coverage(t.Context(), "p.cov"); got.Status != check.Skipped {
		t.Errorf("go off coverage = %+v", got)
	}
	p := gatePlan(t, f, "go", check.Scope{Base: "base", Paths: []string{"fx.go"}})
	p.root = t.TempDir() // no module: go list fails, so the whole module is affected
	if set := p.affectedSet(t.Context()); !set.All || !slices.Equal(set.Patterns, []string{"./..."}) {
		t.Errorf("affected = %+v", set)
	}
	if got := moduleAt(p.root); got != "" {
		t.Errorf("module = %q", got)
	}
	if got := p.packageDir(checkModule + "/internal/sub"); got != "internal/sub" {
		t.Errorf("package dir = %q", got)
	}
}

func TestGenerateGate(t *testing.T) {
	t.Parallel()
	open, root := newPort(t)
	declared, repo := genApp(t)
	for _, tc := range []struct {
		name   string
		ta     testApp
		root   string
		scope  check.Scope
		policy model.GeneratePolicy
		status check.Status
		want   string
	}{
		{"red-an-open-hole", open, root, check.Scope{Whole: true}, model.GeneratePolicy{}, check.Red, "hole open "},
		{"near-a-declared-block-and-nothing-open", declared, repo.Root, check.Scope{Base: "base", Paths: []string{"x.go"}}, model.GeneratePolicy{Declared: true}, check.Green, ""},
		{"near-nothing-changed", declared, repo.Root, check.Scope{Base: "base"}, model.GeneratePolicy{Declared: true}, check.Skipped, "nothing changed"},
		{"near-no-generation", declared, repo.Root, check.Scope{Whole: true}, model.GeneratePolicy{}, check.Skipped, "declares no generation"},
	} {
		p := &checkPlan{a: tc.ta.app, root: tc.root, policy: &model.EffectivePolicy{Generate: tc.policy}, scope: tc.scope}
		got := only(t, p.generateGate(t.Context()))
		text := got.Reason + strings.Join(got.Failures, "\n")
		if got.Status != tc.status || !strings.Contains(text, tc.want) || strings.Contains(text, "open holes") {
			t.Errorf("%s: result = %+v", tc.name, got)
		}
	}
	failing := &checkPlan{a: declared.app, root: filepath.Join(repo.Root, "absent"), policy: &model.EffectivePolicy{Generate: model.GeneratePolicy{Declared: true}}, scope: check.Scope{Whole: true}}
	failing.a.discover = func(context.Context, string) (string, error) { return "", errors.ErrUnsupported }
	if got := only(t, failing.generateGate(t.Context())); got.Status != check.Red {
		t.Errorf("a repository gen cannot find = %+v", got)
	}
}

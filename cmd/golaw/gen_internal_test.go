package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/runnertest"
)

// newPort runs golaw new for the Ledger port and commits the result, the
// state every gen test starts from.
func newPort(t *testing.T) (testApp, string) {
	t.Helper()
	ta, repo := genApp(t)
	if code := ta.run(t.Context(), []string{"new", "port", "growth.Ledger", "--usecase", "ReadPlan"}); code != exitClean {
		t.Fatalf("new exit = %d: %s", code, ta.stderr.String())
	}
	repo.Git("add", "-A")
	repo.Commit("feat: add the ledger port", nil)
	ta.stdout.Reset()
	ta.stderr.Reset()
	return ta, repo.Root
}

func writeRepoFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestGenAfterNewChangesNothing(t *testing.T) {
	t.Parallel()
	ta, _ := newPort(t)
	if code := ta.run(t.Context(), []string{"gen"}); code != exitClean {
		t.Fatalf("exit = %d: %s%s", code, ta.stdout.String(), ta.stderr.String())
	}
	if first, _, _ := strings.Cut(ta.stdout.String(), "\n"); first != "golaw gen: 0 files changed, 0 holes opened, 0 closed, 0 decisions" {
		t.Errorf("stdout = %q", ta.stdout.String())
	}
	if !strings.Contains(ta.stderr.String(), "golaw gen: 0 arquivos alterados, 0 lacunas abertas, 0 fechadas, 0 decisões") {
		t.Errorf("the person's line is missing: %q", ta.stderr.String())
	}
}

func TestGenAppendsTheStubsOfANewPortMethod(t *testing.T) {
	t.Parallel()
	ta, root := newPort(t)
	ports := filepath.Join(root, "internal", "application", "growth", "ports.go")
	data, err := os.ReadFile(ports)
	if err != nil {
		t.Fatal(err)
	}
	method := "\t// PurgePlans deletes the archived plans of one account.\n\tPurgePlans(ctx context.Context, account AccountID) error\n"
	edited := strings.Replace(string(data), "\tArchivePlan(ctx context.Context, account AccountID) error\n", "\tArchivePlan(ctx context.Context, account AccountID) error\n"+method, 1)
	writeRepoFile(t, root, "internal/application/growth/ports.go", edited)
	if code := ta.run(t.Context(), []string{"gen", ports}); code != exitClean {
		t.Fatalf("exit = %d: %s%s", code, ta.stdout.String(), ta.stderr.String())
	}
	out := ta.stdout.String()
	if strings.HasPrefix(out, "golaw gen: 0 files changed") || !strings.Contains(out, "hole open ") || !strings.Contains(out, "purge-plans") {
		t.Errorf("stdout = %s", out)
	}
}

func TestGenNamesAGoneContractAsAPendingDecision(t *testing.T) {
	t.Parallel()
	ta, root := newPort(t)
	if err := os.Remove(filepath.Join(root, "internal", "application", "growth", "ports.go")); err != nil {
		t.Fatal(err)
	}
	// The strict decision line is proven by the engine (GEN-6); here a gone
	// contract must never pass and must be named.
	code := ta.run(t.Context(), []string{"gen"})
	if all := ta.stdout.String() + ta.stderr.String(); code == exitClean || !strings.Contains(all, "internal/application/growth/ports.go") {
		t.Errorf("exit = %d, output %q", code, all)
	}
}

// fakeGenGo puts a fake go command on the binding's PATH that answers the
// path of a fake sqlc, which answers b.
func fakeGenGo(t *testing.T, b runnertest.Behavior) (testApp, string, string) {
	t.Helper()
	ta, root := newPort(t)
	bound := ta.bind()
	bin := t.TempDir()
	sqlcDir := t.TempDir()
	sqlc := runnertest.Install(t, sqlcDir, "sqlc", b)
	runnertest.Install(t, bin, "go", runnertest.Behavior{Stdout: sqlc + "\n"})
	bound.environ = func() []string { return []string{"PATH=" + bin} }
	ta.bind = func() binding { return bound }
	writeRepoFile(t, root, "sqlc.yaml", "version: \"2\"\n")
	writeRepoFile(t, root, "tools/go.mod", "module example.com/slice/tools\n\ngo 1.27\n")
	return ta, root, sqlcDir
}

func TestGenRunsThePinnedGeneratorOfAChangedQuery(t *testing.T) {
	t.Parallel()
	ta, root, sqlcDir := fakeGenGo(t, runnertest.Behavior{Stdout: "ok"})
	if code := ta.run(t.Context(), []string{"gen", filepath.Join(root, "db", "queries", "plans.sql")}); code != exitClean {
		t.Fatalf("exit = %d: %s%s", code, ta.stdout.String(), ta.stderr.String())
	}
	if calls := runnertest.Calls(t, sqlcDir, "sqlc"); !reflect.DeepEqual(calls, [][]string{{"generate", "-f", "sqlc.yaml"}}) {
		t.Errorf("sqlc calls = %v", calls)
	}
	if !strings.Contains(ta.stdout.String(), "ran sqlc sqlc.yaml") {
		t.Errorf("stdout = %s", ta.stdout.String())
	}
}

// The red row of the generator: its failure is exit 2 with its output.
func TestGenNamesAFailingGeneratorWithExitTwo(t *testing.T) {
	t.Parallel()
	ta, _, _ := fakeGenGo(t, runnertest.Behavior{Stderr: "plans.sql:2: syntax error", Code: 1})
	if code := ta.run(t.Context(), []string{"gen"}); code != exitFailure || !strings.Contains(ta.stderr.String(), "plans.sql:2: syntax error") {
		t.Errorf("exit = %d, stderr %q", code, ta.stderr.String())
	}
}

func TestChangedOfReadsAFileInsideTheRepository(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeRepoFile(t, root, "db/queries/plans.sql", "-- name: ReadPlan :one\n")
	tests := map[string][]string{
		filepath.Join(root, "db", "queries", "plans.sql"): {"db/queries/plans.sql"},
		root:                                    nil,
		".":                                     nil,
		filepath.Join(root, "db"):               nil,
		filepath.Join(t.TempDir(), "other.sql"): nil,
	}
	for path, want := range tests {
		if got := changedOf(root, path); !reflect.DeepEqual(got, want) {
			t.Errorf("changedOf(%s) = %v, want %v", path, got, want)
		}
	}
}

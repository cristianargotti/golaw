package main

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/toolchain"
)

// m7LedgerPorts is the Ledger contract of the shared fixture, cut to the lines
// the edit reads.
const m7LedgerPorts = "type Ledger interface {\n" +
	"\t// ArchivePlan archives the plan of one account.\n" +
	"\tArchivePlan(ctx context.Context, account AccountID) error\n" +
	"}\n"

func TestM7PortsWithPurgeAddsOneMethod(t *testing.T) {
	t.Parallel()
	got, err := m7PortsWithPurge([]byte(m7LedgerPorts))
	if err != nil {
		t.Fatal(err)
	}
	want := "\tArchivePlan(ctx context.Context, account AccountID) error\n" +
		"\t// PurgePlans removes the archived plans of one account.\n" +
		"\tPurgePlans(ctx context.Context, account AccountID) error\n}\n"
	if !strings.HasSuffix(string(got), want) {
		t.Fatalf("edited ports:\n%s", got)
	}
	if _, bad := m7PortsWithPurge([]byte("type Clock interface{}\n")); bad == nil {
		t.Fatal("a contract without ArchivePlan was edited")
	}
	if q := string(m7QueriesWithPurge([]byte("-- name: ReadPlan :one\n"))); !strings.HasSuffix(q, "-- name: PurgePlans :execrows\nDELETE FROM plans WHERE account = $1 AND archived;\n") {
		t.Fatalf("edited queries:\n%s", q)
	}
	doc, err := m7OpenAPIWithSummary([]byte("    get:\n      summary: Read the plan of an account\n"))
	if err != nil || string(doc) != "    get:\n      summary: Read the plan of one account\n" {
		t.Fatalf("edited document %q: %v", doc, err)
	}
	if _, bad := m7OpenAPIWithSummary([]byte("openapi: 3.0.3\n")); bad == nil {
		t.Fatal("a document without the GetPlan summary was edited")
	}
}

func TestM7JudgeRegenContextWantsTheSummaryAndTheNewHole(t *testing.T) {
	t.Parallel()
	good := m7RegenSummary + ": 3 files, 0 drift\n" +
		"hole open internal/adapters/out/postgres/plans.go:88 plans.purge-plans PurgePlans: call p.queries.PurgePlans\n" +
		"hole open internal/application/growth/ledgertest/cases.go:61 ledger.purge-plans declare the inputs\n"
	rows := []struct {
		name, text string
		ok         bool
	}{
		{"near the summary and the hole", good, true},
		{"red no summary", strings.SplitN(good, "\n", 2)[1], false},
		{"red the hole of another file", strings.ReplaceAll(good, "postgres/plans.go:88", "postgres/other.go:88"), false},
		{"red nothing", "", false},
	}
	for _, row := range rows {
		if err := m7JudgeRegenContext(row.text); (err == nil) != row.ok {
			t.Fatalf("%s: %v", row.name, err)
		}
	}
}

func TestM7JudgeRegenFilesReadsTheCheckout(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	m2Write(t, filepath.Join(repo, filepath.FromSlash(m7OwnedPlans)), "// golaw:hole plans.purge-plans call p.queries.PurgePlans\n")
	m2Write(t, filepath.Join(repo, filepath.FromSlash(m7FakePath)), "func (f *Fake) PurgePlans() {}\n")
	if err := m7JudgeRegenFiles(repo); err != nil {
		t.Fatal(err)
	}
	m2Write(t, filepath.Join(repo, filepath.FromSlash(m7FakePath)), "func (f *Fake) ArchivePlan() {}\n")
	if err := m7JudgeRegenFiles(repo); err == nil {
		t.Fatal("a fake without the new method passed")
	}
}

func TestM7WithoutRegenerateSwitchesTheLadder(t *testing.T) {
	t.Parallel()
	rows := []struct {
		name, policy, want string
		ok                 bool
	}{
		{"near the key on", "ladder: { pre_commit: true, hook_regenerate: true }\n", "ladder: { pre_commit: true, hook_regenerate: false }\n", true},
		{"near the key absent", "ladder: { pre_commit: true }\n", "ladder: { pre_commit: true, hook_regenerate: false }\n", true},
		{"red no ladder line", "golaw: 1\n", "", false},
	}
	for _, row := range rows {
		got, err := m7WithoutRegenerate([]byte(row.policy))
		if (err == nil) != row.ok || (row.ok && string(got) != row.want) {
			t.Fatalf("%s: %q %v", row.name, got, err)
		}
	}
}

func TestM7JudgeKilledWantsTheNoticeInsideTheBudget(t *testing.T) {
	t.Parallel()
	encode := func(context string, block bool) string {
		var a answer
		a.Hook.Additional = context
		if block {
			a.Decision = decisionBlock
		}
		body, err := json.Marshal(a)
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	notice := "sqlc " + m7NotRegenerated + ": sqlc killed at 1.5s"
	rows := []struct {
		name    string
		elapsed time.Duration
		got     result
		ok      bool
	}{
		{"near killed and allowed", toolchain.HookToolBudget + 200*time.Millisecond, result{stdout: encode(notice, false)}, true},
		{"red answered before the kill", time.Second, result{stdout: encode(notice, false)}, false},
		{"red past the event budget", toolchain.HookEventBudget + 2*time.Second, result{stdout: encode(notice, false)}, false},
		{"red a denial", toolchain.HookToolBudget + time.Second, result{stdout: encode(notice, true)}, false},
		{"red no notice", toolchain.HookToolBudget + time.Second, result{stdout: encode("", false)}, false},
		{"red a failed exit", toolchain.HookToolBudget + time.Second, result{code: 2, stderr: "golaw: hook failed"}, false},
	}
	for _, row := range rows {
		if err := m7JudgeKilled(row.elapsed, row.got); (err == nil) != row.ok {
			t.Fatalf("%s: %v", row.name, err)
		}
	}
}

func TestM7RegenCasesAreInTheCatalog(t *testing.T) {
	t.Parallel()
	want := []string{
		"claude-write", "claude-edit", "codex-apply-patch", "claude-bash-heredoc", "cursor-post",
		"regenerate-off", "port-p95", "query-p95", "openapi-p95", "slow-generator-killed",
	}
	for _, catalog := range m7Catalogs() {
		if catalog.category != m7RegenCategory {
			continue
		}
		if got := m7Names(catalog.cases); !slices.Equal(got, want) {
			t.Fatalf("m7-regen rows = %q", got)
		}
		for _, c := range catalog.cases {
			if c.gen != slices.Contains([]string{"port-p95", "query-p95", "openapi-p95"}, c.name) {
				t.Fatalf("%s: only the budget rows run with the built generators and golangci-lint", c.name)
			}
		}
		return
	}
	t.Fatal("m7Catalogs lacks m7-regen")
}

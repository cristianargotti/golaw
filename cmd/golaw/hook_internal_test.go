package main

import (
	"context"
	"encoding/json"
	"maps"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/checks/text"
	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/agents"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// hookFakes are hook dependencies over in-memory snapshots: an enrolled
// repository, the proof policy, one base file and the dashes check.
func hookFakes(t *testing.T, root string) hook.Dependencies {
	t.Helper()
	b := engine.New()
	if err := b.Register(text.Dashes()); err != nil {
		t.Fatal(err)
	}
	registry, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	base := map[string][]byte{"docs/a.md": []byte("title\n")}
	return hook.Dependencies{
		Enroll: func(context.Context, string, string) (hook.Repository, error) {
			return hook.Repository{Root: root, Identity: root, Enrollment: model.Enrollment{State: model.EnrollmentEnrolled, PolicyDigest: "proof"}}, nil
		},
		Policy:   func(context.Context, hook.Repository) (*model.EffectivePolicy, error) { return proof.Policy(), nil },
		Snapshot: func(context.Context, hook.Repository) (model.Snapshot, error) { return proof.NewTree(base), nil },
		Overlay: func(_ context.Context, _ model.Snapshot, _ []model.Candidate, content map[string][]byte) (model.Snapshot, error) {
			files := map[string][]byte{}
			maps.Copy(files, base)
			maps.Copy(files, content)
			return proof.NewTree(files), nil
		},
		Registry: registry,
		State:    hook.Store{Dir: t.TempDir(), Now: time.Date(2026, time.October, 14, 12, 0, 0, 0, time.UTC)},
		Budget:   time.Second,
	}
}

// writeEvent is a pre tool use write of the agent in its JSON envelope.
func writeEvent(t *testing.T, root, content string) string {
	t.Helper()
	data, err := json.Marshal(map[string]any{
		"session_id": "session", "agent_id": "agent", "tool_use_id": "use", "cwd": root,
		"hook_event_name": "PreToolUse", "tool_name": "Write",
		"tool_input": map[string]any{"file_path": filepath.Join(root, "docs", "b.md"), "content": content},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// hookApp wires the hook command with the fakes over one repository root
// and puts the event the builder makes for that root on stdin.
func hookApp(t *testing.T, event func(root string) string) testApp {
	t.Helper()
	ta := newTestApp(t)
	root := t.TempDir()
	deps := hookFakes(t, root)
	ta.hookDeps = func(agents.Agent, time.Time) (hook.Dependencies, error) { return deps, nil }
	ta.stdin = strings.NewReader(event(root))
	return ta
}

// raw puts fixed bytes on stdin whatever the root is.
func raw(input string) func(string) string {
	return func(string) string { return input }
}

// written builds a write event of the content at the root.
func written(t *testing.T, content string) func(string) string {
	t.Helper()
	return func(root string) string { return writeEvent(t, root, content) }
}

func TestHookDeniesADashAndAllowsTheControl(t *testing.T) {
	t.Parallel()
	id := agents.IDs()[0]
	denied := hookApp(t, written(t, "one \u2014 two\n"))
	if got := denied.run(t.Context(), []string{"hook", id}); got != exitClean {
		t.Fatalf("deny exit = %d, stderr = %q", got, denied.stderr.String())
	}
	if out := denied.stdout.String(); !strings.Contains(out, `"permissionDecision":"deny"`) || !strings.Contains(out, "text.dashes") || denied.stderr.Len() != 0 {
		t.Fatalf("deny stdout = %q, stderr = %q", out, denied.stderr.String())
	}
	allowed := hookApp(t, written(t, "one - two\n"))
	if got := allowed.run(t.Context(), []string{"hook", id}); got != exitClean || allowed.stdout.Len() != 0 || allowed.stderr.Len() != 0 {
		t.Fatalf("allow exit = %d, stdout = %q, stderr = %q", got, allowed.stdout.String(), allowed.stderr.String())
	}
}

func TestHookCannotJudgeMalformedInputWithExitTwo(t *testing.T) {
	t.Parallel()
	ta := hookApp(t, raw("{"))
	if got := ta.run(t.Context(), []string{"hook", agents.IDs()[0]}); got != exitFailure || ta.stdout.Len() != 0 || ta.stderr.Len() == 0 {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", got, ta.stdout.String(), ta.stderr.String())
	}
}

func TestHookRefusesAnUnknownAgentAndReportsSeamFailures(t *testing.T) {
	t.Parallel()
	ta := hookApp(t, raw("{}"))
	if got := ta.run(t.Context(), []string{"hook", "nobody"}); got != exitFailure || !strings.Contains(ta.stderr.String(), `unknown agent "nobody"`) {
		t.Fatalf("exit = %d, stderr = %q", got, ta.stderr.String())
	}
	broken := hookApp(t, raw("{}"))
	broken.hookDeps = func(agents.Agent, time.Time) (hook.Dependencies, error) { return hook.Dependencies{}, errBoom }
	if got := broken.run(t.Context(), []string{"hook", agents.IDs()[0]}); got != exitFailure || broken.stderr.String() != "golaw: hook: boom\n" {
		t.Fatalf("exit = %d, stderr = %q", got, broken.stderr.String())
	}
	if got := ta.run(t.Context(), []string{"hook"}); got != exitFailure {
		t.Fatalf("exit without an agent = %d", got)
	}
}

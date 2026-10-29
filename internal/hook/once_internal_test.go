package hook

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

func TestAnEventWithoutASessionSaysEverythingAgain(t *testing.T) {
	t.Parallel()
	if _, ok := saidKey(Repository{Identity: "repository"}, model.Event{Agent: "claude"}, "missing:ruff"); ok {
		t.Fatal("a key without a session id")
	}
	dir := t.TempDir()
	run := execution{request: Request{Event: model.Event{Agent: "claude"}}}
	deps := Dependencies{State: Store{Dir: dir, Now: time.Now()}}
	for range 2 {
		if run.said(t.Context(), "missing:ruff", deps) {
			t.Fatal("an event without a session was remembered")
		}
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
		t.Fatalf("state entries = %v, %v, want none", entries, err)
	}
}

// A state that cannot be read repeats the line rather than losing it.
func TestAStateThatCannotBeReadRepeatsTheLine(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "state")
	if err := os.WriteFile(dir, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	run := execution{request: Request{Event: model.Event{Agent: "claude", SessionID: "session"}}}
	deps := Dependencies{State: Store{Dir: dir, Now: time.Now()}}
	for range 2 {
		if run.said(t.Context(), "missing:ruff", deps) {
			t.Fatal("an unreadable state dropped a notice")
		}
	}
}

// A finding whose path the snapshot cannot read keeps its line.
func TestAnUnreadablePathKeepsItsLines(t *testing.T) {
	t.Parallel()
	run := execution{request: Request{Event: model.Event{Agent: "claude", SessionID: "session"}}}
	deps := Dependencies{State: Store{Dir: t.TempDir(), Now: time.Now()}}
	in := model.Input{Snapshot: proof.NewTree(nil)}
	f := fed{findings: []model.Finding{{Check: "docker.final-user", Path: "gone/Dockerfile"}}}
	for range 2 {
		if got := run.once(t.Context(), f, in, nil, deps); len(got.findings) != 1 {
			t.Fatalf("findings = %+v, want the line kept", got.findings)
		}
	}
}

// A decision whose result cannot judge drops the feedback, so the session
// remembers none of it.
func TestOnlyADeliveredDecisionIsRemembered(t *testing.T) {
	t.Parallel()
	unmeasured := engine.Decision{Verdict: engine.VerdictDeny, Reason: "surface.code-eval: unmeasured", Unmeasured: []engine.Unmeasured{{Check: "surface.code-eval"}}}
	if delivered(unmeasured) || delivered(engine.Decision{Verdict: engine.VerdictDeny}) {
		t.Fatal("a result that cannot judge was taken as delivered")
	}
	if !delivered(engine.Decision{Verdict: engine.VerdictClean}) || !delivered(engine.Decision{Verdict: engine.VerdictDeny, Reason: "docker.final-user: root"}) {
		t.Fatal("a delivered result was refused")
	}
}

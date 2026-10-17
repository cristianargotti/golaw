package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// slowRead is the snapshot of the real binding whose read of one file
// waits for the deadline of the event first, as a read from a slow disk
// would, recording that the run reached it.
type slowRead struct {
	model.Snapshot

	done    <-chan struct{}
	slow    string
	entered *bool
}

// Read waits for the deadline before it reads the slow file.
func (s slowRead) Read(path string) ([]byte, error) {
	if path == s.slow {
		*s.entered = true
		<-s.done
	}
	return s.Snapshot.Read(path) //nolint:wrapcheck // the snapshot of the binding names the path in its error
}

// withSlowRead gives the dependencies a snapshot whose read of the slow
// file waits for the deadline of the event and a budget the hook spends
// there.
func withSlowRead(deps hook.Dependencies, slow string, entered *bool) hook.Dependencies {
	snapshot := deps.Snapshot
	deps.Snapshot = func(ctx context.Context, repo hook.Repository) (model.Snapshot, error) {
		base, err := snapshot(ctx, repo)
		if err != nil {
			return nil, err
		}
		return slowRead{Snapshot: base, done: ctx.Done(), slow: slow, entered: entered}, nil
	}
	deps.Budget = 3 * time.Second
	return deps
}

// TestHookKeepsTheFindingsOfACheckTheDeadlineStopped drives the post event
// of a command that wrote three Go packages through the real binding, where
// the read of a committed test file only goast.goleak reads waits for the
// deadline: the deadline lands inside that check between its targets, after
// its finding for the first package, and that finding is fed back with the
// deadline notice appended.
func TestHookKeepsTheFindingsOfACheckTheDeadlineStopped(t *testing.T) { //nolint:paralleltest // the budget measures the hook without competing binding tests
	agent := agentUnderTest()
	e := enrolledHook(t)
	commitFiles(t, e.root, map[string]string{"p2/b_test.go": "package p2\n\nimport \"testing\"\n\nfunc TestStart(t *testing.T) { t.Log(\"start\") }\n"})
	pre := hook.Run(t.Context(), agent.ID, strings.NewReader(shellEventAt(t, "PreToolUse", e.root, "true")), agent.Adapter, e.deps)
	start := "\n\n// Start starts itself again.\nfunc Start() { go Start() }\n"
	writeFiles(t, e.root, map[string]string{
		"p1/a.go": "package p1" + start, "p2/b.go": "package p2" + start,
		"p3/c.go": "package p3\n\n// Value is a value.\nconst Value = 1\n",
	})
	var entered bool
	deps := withSlowRead(e.deps, "p2/b_test.go", &entered)
	reply := hook.Run(t.Context(), agent.ID, strings.NewReader(shellEventAt(t, "PostToolUse", e.root, "true")), agent.Adapter, deps)
	var answer struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	if err := json.Unmarshal(reply.Stdout, &answer); err != nil || pre.ExitCode != 0 || !entered {
		t.Fatalf("pre %+v, post %+v, entered %v: %v", pre, reply, entered, err)
	}
	want := "goast.goleak: package p1 starts goroutines and no test of the package verifies with goleak (p1/a.go:4) " + postDeadlineNotice
	if reply.ExitCode != 0 || answer.Decision != "block" || answer.Reason != want {
		t.Fatalf("a finding of a check the deadline stopped: exit %d, stdout %q, stderr %q", reply.ExitCode, reply.Stdout, reply.Stderr)
	}
}

package hook_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// errRecord is the failure of a capture record the answer must survive.
var errRecord = errors.New("disk full")

// eventProbe denies every event it judges, so a request the core judges is
// told from one it only observes.
type eventProbe struct{}

func (eventProbe) Spec() model.Spec {
	return model.Spec{
		ID: "shell.probe", Rule: "rules/3@1", Scope: model.ScopeEvent,
		Owner: model.OwnerCore, Proof: "shell.probe",
	}
}

func (eventProbe) Run(context.Context, model.Input) (model.Result, error) {
	return model.Result{Status: model.Measured(), Findings: []model.Finding{{
		Code: "probe-event", Message: "judged event", Severity: model.SeverityDeny,
	}}}, nil
}

// hostShell is a pre event of a host shell tool the core does not know
// by name, with the identities A15 keys a baseline by.
func hostShell(tool string) hook.Request {
	return hook.Request{Event: model.Event{
		Agent: "cursor", Tool: tool, Shape: model.ShapeShell,
		SessionID: "conversation", AgentID: "generation", ToolUseID: "tool",
		Shell: &model.ShellCommand{Command: "printf changed > a.go", Tool: tool},
	}}
}

func judgingEvents(t *testing.T, dependencies hook.Dependencies) hook.Dependencies {
	t.Helper()
	builder := engine.New()
	for _, check := range []model.Check{probeCheck{}, eventProbe{}} {
		if err := builder.Register(check); err != nil {
			t.Fatal(err)
		}
	}
	registry, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	dependencies.Registry = registry
	return dependencies
}

func TestObserveKeepsTheBaselineAndAllowsWithoutJudging(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		observe bool
		want    hook.Outcome
	}{
		{"red observed", true, hook.Allow},
		{"near judged", false, hook.Deny},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := hostShell("Shell")
			req.Observe = tc.observe
			adapter, dependencies := harness(t, req)
			dependencies = judgingEvents(t, dependencies)
			event, got := hook.Evaluate(t.Context(), "cursor", []byte("{}"), adapter, dependencies)
			if got.Outcome != tc.want {
				t.Fatalf("%+v", got)
			}
			key, err := hook.StateKey(event.Cwd, event)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := dependencies.State.Load(t.Context(), key); tc.observe && err != nil {
				t.Fatalf("no baseline kept: %v", err)
			}
		})
	}
}

func TestObserveWithoutToolUseKeepsNoBaseline(t *testing.T) {
	t.Parallel()
	req := hostShell("Shell")
	req.Observe, req.Event.ToolUseID = true, ""
	adapter, dependencies := harness(t, req)
	event, got := hook.Evaluate(t.Context(), "cursor", []byte("{}"), adapter, dependencies)
	if got.Outcome != hook.Allow {
		t.Fatalf("%+v", got)
	}
	event.ToolUseID = "tool"
	key, err := hook.StateKey(event.Cwd, event)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dependencies.State.Load(t.Context(), key); err == nil {
		t.Fatal("a baseline was kept without its tool use id")
	}
}

func TestDetectRunsTheDetectionPassOfAnUnknownTool(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		detect bool
		want   hook.Outcome
	}{
		{"red detected", true, hook.Feedback},
		{"near not detected", false, hook.Allow},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := hostShell("Shell")
			req.Observe = true
			adapter, dependencies := harness(t, req)
			if _, before := hook.Evaluate(t.Context(), "cursor", []byte("{}"), adapter, dependencies); before.Outcome != hook.Allow {
				t.Fatal(before)
			}
			post := hook.Request{Event: req.Event, Detect: tc.detect}
			post.Event.Shape, post.Event.Shell = model.ShapePost, nil
			post.Event.Post = &model.PostResult{Response: "{}"}
			adapter.Parse = parsing(post.Event, func(hook.Base, model.Snapshot) (hook.Request, error) {
				return post, nil
			})
			dependencies.Snapshot = func(context.Context, hook.Repository) (model.Snapshot, error) {
				return proof.NewTree(map[string][]byte{"a.go": []byte("bad")}), nil
			}
			_, after := hook.Evaluate(t.Context(), "cursor", []byte("{}"), adapter, dependencies)
			if after.Outcome != tc.want {
				t.Fatalf("%+v", after)
			}
		})
	}
}

func TestCaptureIsRecordedBeforeTheAnswer(t *testing.T) {
	t.Parallel()
	capture := hook.Capture{Event: "preToolUse", Tool: "Write", Keys: []string{"content", "file_path"}}
	for _, tc := range []struct {
		name    string
		capture *hook.Capture
		fail    error
		calls   int
	}{
		{"red recorded", &capture, nil, 1},
		{"red failed record still allows", &capture, errRecord, 1},
		{"near nothing to record", nil, nil, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := hostShell("Write")
			req.Observe, req.Capture = true, tc.capture
			adapter, dependencies := harness(t, req)
			var got []hook.Capture
			dependencies.Capture = func(_ context.Context, repository hook.Repository, c hook.Capture) error {
				if repository.Identity == "" {
					t.Error("the capture lost its repository")
				}
				got = append(got, c)
				return tc.fail
			}
			_, result := hook.Evaluate(t.Context(), "cursor", []byte("{}"), adapter, dependencies)
			if result.Outcome != hook.Allow || len(got) != tc.calls {
				t.Fatalf("%+v %v", result, got)
			}
			if tc.calls == 1 && (got[0].Tool != "Write" || !slices.Equal(got[0].Keys, capture.Keys)) {
				t.Fatalf("%+v", got[0])
			}
		})
	}
}

func TestCaptureWaitsForEnrollment(t *testing.T) {
	t.Parallel()
	req := hostShell("Write")
	req.Observe, req.Capture = true, &hook.Capture{Event: "preToolUse", Tool: "Write", Keys: []string{"path"}}
	adapter, dependencies := harness(t, req)
	dependencies.Enroll = func(context.Context, string, string) (hook.Repository, error) {
		return hook.Repository{Enrollment: model.Enrollment{State: model.EnrollmentNone}}, nil
	}
	called := false
	dependencies.Capture = func(context.Context, hook.Repository, hook.Capture) error {
		called = true
		return nil
	}
	if _, got := hook.Evaluate(t.Context(), "cursor", []byte("{}"), adapter, dependencies); got.Outcome != hook.Allow || called {
		t.Fatalf("%+v recorded=%v", got, called)
	}
}

package hook_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

func TestEnrollmentPrecedesPolicyAndTargets(t *testing.T) {
	t.Parallel()
	adapter, dependencies := harness(t, request("bad"))
	dependencies.Enroll = func(context.Context, string) (hook.Repository, error) {
		return hook.Repository{
			Enrollment: model.Enrollment{State: model.EnrollmentNone},
		}, nil
	}
	dependencies.Policy = func(context.Context, hook.Repository) (*model.EffectivePolicy, error) {
		t.Fatal("policy read before positive enrollment")
		return nil, model.ErrCannotJudge
	}
	dependencies.Snapshot = func(context.Context, hook.Repository) (model.Snapshot, error) {
		t.Fatal("target read before positive enrollment")
		return nil, model.ErrCannotJudge
	}
	_, got := hook.Evaluate(t.Context(), "claude", []byte("{}"), adapter, dependencies)
	if got.Outcome != hook.Allow {
		t.Fatal(got)
	}
}

func TestEngineReasonAndPolicyCause(t *testing.T) {
	t.Parallel()
	adapter, dependencies := harness(t, request("bad"))
	_, got := hook.Evaluate(t.Context(), "claude", []byte("{}"), adapter, dependencies)
	if got.Outcome != hook.Deny || got.Reason != "text.probe: invalid text (a.go)" ||
		!strings.Contains(got.Human, "text.probe") || !strings.Contains(got.Human, "a.go") {
		t.Fatal(got)
	}
	for _, tc := range []struct {
		state model.EnrollmentState
		cause string
	}{
		{model.EnrollmentMissing, "missing golaw.yml"},
		{model.EnrollmentMismatch, "golaw.yml enrollment digest mismatch"},
		{model.EnrollmentNone, "unenrolled"},
	} {
		policy := proof.Policy()
		policy.Enrollment.State = tc.state
		dependencies.Policy = func(context.Context, hook.Repository) (*model.EffectivePolicy, error) {
			return policy, nil
		}
		_, got = hook.Evaluate(t.Context(), "claude", []byte("{}"), adapter, dependencies)
		if got.Outcome != hook.Deny || got.CannotJudge || !strings.Contains(got.Reason, tc.cause) {
			t.Fatal(got)
		}
	}
	dependencies.Policy = func(context.Context, hook.Repository) (*model.EffectivePolicy, error) {
		return nil, fmt.Errorf("%w: file_lines weakens 300 to 301", model.ErrPolicy)
	}
	_, got = hook.Evaluate(t.Context(), "claude", []byte("{}"), adapter, dependencies)
	if got.Outcome != hook.Deny || !strings.Contains(got.Reason, "weakens 300 to 301") {
		t.Fatal(got)
	}
	_, got = hook.Evaluate(t.Context(), "cursor", []byte("{}"), adapter, dependencies)
	if !got.CannotJudge {
		t.Fatal("agent mismatch allowed")
	}
}

func TestStaleCandidateAndProtectedPolicy(t *testing.T) {
	t.Parallel()
	adapter, dependencies := harness(t, request("safe"))
	calls := 0
	dependencies.Snapshot = func(context.Context, hook.Repository) (model.Snapshot, error) {
		calls++
		content := "safe"
		if calls > 1 {
			content = "changed"
		}
		return proof.NewTree(map[string][]byte{"a.go": []byte(content)}), nil
	}
	_, got := hook.Evaluate(t.Context(), "claude", []byte("{}"), adapter, dependencies)
	if !got.CannotJudge || !strings.Contains(got.Reason, "base changed") {
		t.Fatal(got)
	}
	req := request("safe")
	req.Event.Candidates[0].Path = "golaw.yml"
	adapter, dependencies = harness(t, req)
	_, got = hook.Evaluate(t.Context(), "claude", []byte("{}"), adapter, dependencies)
	if got.Outcome != hook.Deny || !strings.Contains(got.Reason, "golaw.yml") {
		t.Fatal(got)
	}
}

func TestProtectedPolicyFoldsCaseOnACaseInsensitiveRepository(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		path               string
		insensitive, wants bool
	}{
		{"GOLAW.yml", true, true},
		{"GOLAW.yml", false, false},
	} {
		req := request("safe")
		req.Event.Candidates[0].Path = tc.path
		adapter, dependencies := harness(t, req)
		enroll := dependencies.Enroll
		dependencies.Enroll = func(ctx context.Context, cwd string) (hook.Repository, error) {
			repository, err := enroll(ctx, cwd)
			repository.CaseInsensitive = tc.insensitive
			return repository, err
		}
		_, got := hook.Evaluate(t.Context(), "claude", []byte("{}"), adapter, dependencies)
		if refused := got.Outcome == hook.Deny && strings.Contains(got.Reason, "golaw.yml"); refused != tc.wants {
			t.Fatal(tc, got)
		}
	}
}

func TestShellPostUsesContentIdentity(t *testing.T) {
	t.Parallel()
	req := request("safe")
	req.Event.Shape, req.Event.Tool = model.ShapeShell, "Bash"
	req.Event.Candidates = nil
	req.Event.Shell = &model.ShellCommand{Command: "printf changed > a.go; false", Tool: "Bash"}
	adapter, dependencies := harness(t, req)
	_, before := hook.Evaluate(t.Context(), "claude", []byte("{}"), adapter, dependencies)
	if before.Outcome != hook.Allow {
		t.Fatal(before)
	}
	req.Event.Shape = model.ShapePost
	req.Event.Shell = nil
	req.Event.Post = &model.PostResult{Failed: true, Response: "command failed"}
	adapter.Header = func([]byte) (model.Event, error) { return req.Event, nil }
	adapter.Decode = func([]byte, string, model.Snapshot) (hook.Request, error) {
		return req, nil
	}
	dependencies.Snapshot = func(context.Context, hook.Repository) (model.Snapshot, error) {
		return proof.NewTree(map[string][]byte{"a.go": []byte("bad")}), nil
	}
	_, after := hook.Evaluate(t.Context(), "claude", []byte("{}"), adapter, dependencies)
	if after.Outcome != hook.Feedback || after.Reason != "text.probe: invalid text (a.go)" {
		t.Fatal(after)
	}
}

func TestDecisionAndSelectionMutations(t *testing.T) {
	t.Parallel()
	adapter, dependencies := harness(t, request("bad"))
	_, baseline := hook.Evaluate(t.Context(), "claude", []byte("{}"), adapter, dependencies)
	if baseline.Outcome != hook.Deny {
		t.Fatal(baseline)
	}
	adapter.Decode = func([]byte, string, model.Snapshot) (hook.Request, error) {
		return hook.Request{Event: model.Event{
			Agent: "claude", Tool: "Read", Shape: model.ShapeRead,
			Read: &model.ReadRequest{Path: "a.go"},
		}}, nil
	}
	_, deleted := hook.Evaluate(t.Context(), "claude", []byte("{}"), adapter, dependencies)
	if deleted.Outcome != hook.Allow {
		t.Fatal("selection deletion did not remove the red finding")
	}
	adapter, dependencies = harness(t, request("safe"))
	adapter.Decode = func([]byte, string, model.Snapshot) (hook.Request, error) {
		return hook.Request{Denial: "forced denial"}, nil
	}
	_, overfire := hook.Evaluate(t.Context(), "claude", []byte("{}"), adapter, dependencies)
	if overfire.Outcome != hook.Deny {
		t.Fatal("near miss did not expose forced denial")
	}
	got := hook.DecisionResult(model.Event{}, engine.Decision{
		Verdict: engine.VerdictDeny, Reason: "check: not measured: unreadable",
		Unmeasured: []engine.Unmeasured{{Check: "check", Reason: "unreadable"}},
	})
	if !got.CannotJudge || got.Reason != "check: not measured: unreadable" {
		t.Fatal(got)
	}
}

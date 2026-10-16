package hook_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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
	dependencies.Enroll = func(context.Context, string, string) (hook.Repository, error) {
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

func TestProtectedPolicyCaseAndMoveSource(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		path, from         string
		insensitive, wants bool
	}{
		{"GOLAW.yml", "", true, true},
		{"GOLAW.yml", "", false, false},
		{"b.go", "golaw.yml", false, true},
	} {
		req := request("safe")
		req.Event.Candidates[0].Path = tc.path
		if tc.from != "" {
			req.Event.Candidates[0].Op, req.Event.Candidates[0].From = model.CandidateMove, tc.from
		}
		adapter, dependencies := harness(t, req)
		enroll := dependencies.Enroll
		dependencies.Enroll = func(ctx context.Context, dir, cwd string) (hook.Repository, error) {
			repository, err := enroll(ctx, dir, cwd)
			repository.CaseInsensitive = tc.insensitive
			return repository, err
		}
		_, got := hook.Evaluate(t.Context(), "claude", []byte("{}"), adapter, dependencies)
		if refused := got.Outcome == hook.Deny && strings.Contains(got.Reason, "golaw.yml"); refused != tc.wants {
			t.Fatal(tc, got)
		}
	}
}

func TestEnrollFailureAndUnknownAgentCannotBeJudged(t *testing.T) {
	t.Parallel()
	adapter, dependencies := harness(t, request("safe"))
	dependencies.Enroll = func(context.Context, string, string) (hook.Repository, error) {
		return hook.Repository{}, fmt.Errorf("%w: discovery failed", model.ErrCannotJudge)
	}
	_, got := hook.Evaluate(t.Context(), "claude", []byte("{}"), adapter, dependencies)
	if got.Outcome != hook.Deny || !got.CannotJudge || !strings.Contains(got.Reason, "discovery failed") {
		t.Fatal(got)
	}
	req := request("safe")
	req.Event.Agent = "other"
	adapter, dependencies = harness(t, req)
	_, got = hook.Evaluate(t.Context(), "other", []byte("{}"), adapter, dependencies)
	if got.Outcome != hook.Deny || !got.CannotJudge {
		t.Fatal("an unknown agent was judged:", got)
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
	adapter.Parse = parsing(req.Event, func(hook.Base, model.Snapshot) (hook.Request, error) {
		return req, nil
	})
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
	adapter = decoded(adapter, hook.Request{Event: model.Event{
		Agent: "claude", Tool: "Read", Shape: model.ShapeRead,
		Read: &model.ReadRequest{Path: "a.go"},
	}})
	_, deleted := hook.Evaluate(t.Context(), "claude", []byte("{}"), adapter, dependencies)
	if deleted.Outcome != hook.Allow {
		t.Fatal("selection deletion did not remove the red finding")
	}
	adapter, dependencies = harness(t, request("safe"))
	adapter = decoded(adapter, hook.Request{Denial: "forced denial"})
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

// TestEnrollmentFollowsTheTarget holds enrollment to the repository that
// holds the file a write or read names, wherever the session runs: the
// deepest existing directory above the target is discovered, and the
// session's working directory is kept for the shell shape and as evidence.
func TestEnrollmentFollowsTheTarget(t *testing.T) {
	t.Parallel()
	session, repo := realDir(t), realDir(t)
	for _, dir := range []string{filepath.Join(repo, "internal"), filepath.Join(session, "sub")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "internal", "a.go"), []byte("safe"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, target, want string
	}{
		{"existing file", filepath.Join(repo, "internal", "a.go"), filepath.Join(repo, "internal")},
		{"absent directories", filepath.Join(repo, "new", "deeper", "b.go"), repo},
		{"relative to the session", filepath.Join("sub", "c.go"), filepath.Join(session, "sub")},
		{"absent relative directory", filepath.Join("gone", "d.go"), session},
		{"no target", "", session},
	} {
		req := request("bad")
		req.Event.Cwd, req.Event.Target = session, tc.target
		dependencies := harnessDependencies(t, repo)
		var dir, cwd string
		dependencies.Enroll = func(_ context.Context, d, c string) (hook.Repository, error) {
			dir, cwd = d, c
			return hook.Repository{Enrollment: model.Enrollment{State: model.EnrollmentNone}}, nil
		}
		_, got := hook.Evaluate(t.Context(), "claude", []byte("{}"), harnessAdapter(req), dependencies)
		if got.Outcome != hook.Allow || dir != tc.want || cwd != session {
			t.Fatalf("%s: enrolled from %q with cwd %q, %+v", tc.name, dir, cwd, got)
		}
	}
}

// TestReadOutsideEveryRepositoryFallsBackToTheSession keeps the secret
// boundary of an enrolled session: a read whose target no enrolled
// repository holds is judged by the session's repository, while a write
// there is a positively unenrolled location that no policy loads for.
func TestReadOutsideEveryRepositoryFallsBackToTheSession(t *testing.T) {
	t.Parallel()
	session, outside := realDir(t), realDir(t)
	for _, tc := range []struct {
		shape  model.EventShape
		calls  []string
		judged bool
	}{
		{model.ShapeRead, []string{outside, session}, true},
		{model.ShapeWrite, []string{outside}, false},
	} {
		req := request("bad")
		req.Event.Shape, req.Event.Cwd, req.Event.Target = tc.shape, session, filepath.Join(outside, "file")
		if tc.shape == model.ShapeRead {
			req.Event.Candidates, req.Content = nil, nil
			req.Event.Read = &model.ReadRequest{Path: req.Event.Target}
		}
		dependencies := harnessDependencies(t, session)
		enroll, policy := dependencies.Enroll, dependencies.Policy
		var calls []string
		dependencies.Enroll = func(ctx context.Context, dir, cwd string) (hook.Repository, error) {
			calls = append(calls, dir)
			if dir != session {
				return hook.Repository{Enrollment: model.Enrollment{State: model.EnrollmentNone}}, nil
			}
			return enroll(ctx, dir, cwd)
		}
		judged := false
		dependencies.Policy = func(ctx context.Context, repository hook.Repository) (*model.EffectivePolicy, error) {
			judged = true
			return policy(ctx, repository)
		}
		_, got := hook.Evaluate(t.Context(), "claude", []byte("{}"), harnessAdapter(req), dependencies)
		if !slices.Equal(calls, tc.calls) || judged != tc.judged || got.Outcome != hook.Allow {
			t.Fatalf("%s: enrolled from %v, judged %v, %+v", tc.shape, calls, judged, got)
		}
	}
}

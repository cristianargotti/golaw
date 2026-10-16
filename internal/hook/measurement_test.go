package hook_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// asPostEvent keeps the identities of the shell request, so the post event
// finds the state its pre event saved.
func asPostEvent(req *hook.Request, cwd string, failed bool) hook.Adapter {
	req.Event.Cwd = cwd
	req.Event.Shape, req.Event.Shell = model.ShapePost, nil
	req.Event.Post = &model.PostResult{Failed: failed}
	adapter := harnessAdapter(*req)
	adapter.Header = func([]byte) (model.Event, error) { return req.Event, nil }
	adapter.Decode = func([]byte, hook.Base, model.Snapshot) (hook.Request, error) {
		return *req, nil
	}
	return adapter
}

// savedReason reads the store directly, because the allow of the pre event
// never shows whether its capture failed.
func savedReason(t *testing.T, dependencies hook.Dependencies, event model.Event) string {
	t.Helper()
	repository, err := dependencies.Enroll(t.Context(), event.Cwd, event.Cwd)
	if err != nil {
		t.Fatal(err)
	}
	key, err := hook.StateKey(repository.Identity, event)
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := dependencies.State.Load(t.Context(), key)
	if err != nil || baseline.Files != nil {
		t.Fatalf("%+v, %v", baseline, err)
	}
	return baseline.Reason
}

func shellRequest() hook.Request {
	req := request("safe")
	req.Event.Shape, req.Event.Tool = model.ShapeShell, "Bash"
	req.Event.Candidates = nil
	req.Event.Shell = &model.ShellCommand{
		Command: "printf changed > a.go", Tool: "Bash",
	}
	return req
}

// TestShellCaptureFailureDoesNotDeny keeps a failed content capture of a
// pre event out of every verdict: the pre event is allowed with the cause
// saved, and the post event names what it could not measure in a notice
// that blocks nothing, while a file the host reports changed is judged.
func TestShellCaptureFailureDoesNotDeny(t *testing.T) {
	t.Parallel()
	req := shellRequest()
	adapter, dependencies := harness(t, req)
	dependencies.Changes = func(context.Context, hook.Repository, []string) (hook.Changes, error) {
		return hook.Changes{}, model.ErrUnreadable
	}
	event, got := hook.Evaluate(t.Context(), "claude", []byte("{}"), adapter, dependencies)
	if got.Outcome != hook.Allow || got.CannotJudge {
		t.Fatal(got)
	}
	if reason := savedReason(t, dependencies, event); reason == "" {
		t.Fatal("the capture failure was not saved as the reason")
	}
	adapter = asPostEvent(&req, event.Cwd, false)
	dependencies.Changes = nil
	dependencies.Snapshot = func(context.Context, hook.Repository) (model.Snapshot, error) {
		return proof.NewTree(map[string][]byte{"a.go": []byte("bad")}), nil
	}
	_, got = hook.Evaluate(t.Context(), "claude", []byte("{}"), adapter, dependencies)
	if got.Outcome != hook.Allow || got.CannotJudge || !strings.Contains(got.Notice, "the pipeline judges") {
		t.Fatal(got)
	}
	req.Hints = []string{"a.go"}
	_, got = hook.Evaluate(t.Context(), "claude", []byte("{}"), adapter, dependencies)
	if got.Outcome != hook.Feedback || !strings.HasPrefix(got.Reason, "text.probe: invalid text (a.go)") {
		t.Fatal(got)
	}
	dependencies.Snapshot = func(context.Context, hook.Repository) (model.Snapshot, error) {
		return proof.NewTree(map[string][]byte{"a.go": []byte("safe")}), nil
	}
	_, got = hook.Evaluate(t.Context(), "claude", []byte("{}"), adapter, dependencies)
	if got.Outcome != hook.Allow || got.Notice == "" {
		t.Fatal(got)
	}
}

// TestShellStateFailureAndMissingBaselineAreNotices answers a post event
// whose pre event left no usable baseline with an allow and a notice that
// names the cause and leaves the command to the pipeline.
func TestShellStateFailureAndMissingBaselineAreNotices(t *testing.T) {
	t.Parallel()
	req := shellRequest()
	adapter, dependencies := harness(t, req)
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("occupied"), 0o600); err != nil {
		t.Fatal(err)
	}
	dependencies.State.Dir = path
	event, got := hook.Evaluate(t.Context(), "claude", []byte("{}"), adapter, dependencies)
	if got.Outcome != hook.Allow || got.CannotJudge {
		t.Fatal(got)
	}
	adapter = asPostEvent(&req, event.Cwd, true)
	_, got = hook.Evaluate(t.Context(), "claude", []byte("{}"), adapter, dependencies)
	if got.Outcome != hook.Allow || got.CannotJudge || !strings.Contains(got.Notice, "the pipeline judges") {
		t.Fatal(got)
	}
	dependencies.State.Dir = t.TempDir()
	_, got = hook.Evaluate(t.Context(), "claude", []byte("{}"), adapter, dependencies)
	if got.Outcome != hook.Allow || !strings.Contains(got.Notice, "no baseline") || !strings.Contains(got.Human, "pipeline") {
		t.Fatal(got)
	}
}

func TestWriteSavesNoUnusedStateAndLanguageIsThePolicys(t *testing.T) {
	t.Parallel()
	adapter, dependencies := harness(t, request("safe"))
	_, got := hook.Evaluate(t.Context(), "claude", []byte("{}"), adapter, dependencies)
	if got.Outcome != hook.Allow {
		t.Fatal(got)
	}
	entries, err := os.ReadDir(dependencies.State.Dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("%v, %v", entries, err)
	}
	adapter, dependencies = harness(t, request("bad"))
	_, got = hook.Evaluate(t.Context(), "claude", []byte("{}"), adapter, dependencies)
	if got.Human != "golaw bloqueou a operação: violação de regra (verificação text.probe, arquivo a.go)." {
		t.Fatal(got)
	}
	policy := proof.Policy()
	policy.Language = "en-US"
	dependencies.Policy = func(context.Context, hook.Repository) (*model.EffectivePolicy, error) {
		return policy, nil
	}
	_, got = hook.Evaluate(t.Context(), "claude", []byte("{}"), adapter, dependencies)
	if got.Human != "golaw blocked the operation: text.probe: invalid text (a.go)." {
		t.Fatal(got)
	}
	dependencies.Policy = func(context.Context, hook.Repository) (*model.EffectivePolicy, error) {
		return nil, model.ErrEnrollment
	}
	_, got = hook.Evaluate(t.Context(), "claude", []byte("{}"), adapter, dependencies)
	if got.CannotJudge || got.Outcome != hook.Deny ||
		got.Reason != "load hook policy: enrollment mismatch" {
		t.Fatal(got)
	}
}

// TestAFailedListingSpeaksThePolicyLanguage names a worktree listing that
// failed under the decoder of a write, which reads the base, in the
// language of the policy in force.
func TestAFailedListingSpeaksThePolicyLanguage(t *testing.T) {
	t.Parallel()
	adapter, dependencies := harness(t, request("bad"))
	policy := proof.Policy()
	policy.Language = "en-US"
	dependencies.Policy = func(context.Context, hook.Repository) (*model.EffectivePolicy, error) {
		return policy, nil
	}
	dependencies.Snapshot = func(context.Context, hook.Repository) (model.Snapshot, error) {
		return nil, model.ErrUnreadable
	}
	decode := adapter.Decode
	adapter.Decode = func(data []byte, base hook.Base, snapshot model.Snapshot) (hook.Request, error) {
		snapshot.Class("a.go")
		return decode(data, base, snapshot)
	}
	_, got := hook.Evaluate(t.Context(), "claude", []byte("{}"), adapter, dependencies)
	if got.Human != "The operation was blocked by check golaw at path not given." || !got.CannotJudge {
		t.Fatal(got)
	}
}

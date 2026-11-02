package claude_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/claude"
	"github.com/cristianargotti/golaw/internal/hooktest"
	"github.com/cristianargotti/golaw/internal/model"
)

const sessionPayload = `{"session_id":"session","transcript_path":"/project/.transcript.jsonl","cwd":"/project","hook_event_name":"SessionStart","source":"startup"}`

const mapText = "golaw map layout: port internal/application/{area}"

var errUnreadablePolicy = errors.New("unreadable policy")

func sessionAdapter() hook.Adapter {
	return hook.Adapter{Parse: claude.Parse, Encode: claude.Encode, Oversized: claude.Oversized}
}

func sessionDeps(t *testing.T, root string) hook.Dependencies {
	t.Helper()
	deps := hooktest.Dependencies(t, root, map[string][]byte{"go.mod": []byte("module example.com/slice\n")})
	deps.Session = func(context.Context, hook.Repository, *model.EffectivePolicy) string { return mapText }
	return deps
}

func TestSessionStartAnswersTheMapAsContext(t *testing.T) {
	t.Parallel()
	root := hooktest.Repository(t)
	event, result := hook.Evaluate(t.Context(), "claude", hooktest.Payload([]byte(sessionPayload), root), sessionAdapter(), sessionDeps(t, root))
	if event.Shape != model.ShapeSession || result.Outcome != hook.Allow {
		t.Fatalf("event %+v result %+v", event, result)
	}
	reply := claude.Encode(event, result)
	var doc struct {
		Specific struct {
			Event   string `json:"hookEventName"`
			Context string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(reply.Stdout, &doc); err != nil || reply.ExitCode != 0 || doc.Specific.Event != "SessionStart" || doc.Specific.Context != mapText {
		t.Fatalf("reply = %+v (%s), %v", reply, reply.Stdout, err)
	}
}

func TestSessionStartNeverDenies(t *testing.T) {
	t.Parallel()
	root := hooktest.Repository(t)
	payload := hooktest.Payload([]byte(sessionPayload), root)
	unenrolled := sessionDeps(t, root)
	unenrolled.Enroll = func(context.Context, string, string) (hook.Repository, error) {
		return hook.Repository{Enrollment: model.Enrollment{State: model.EnrollmentNone}}, nil
	}
	broken := sessionDeps(t, root)
	broken.Policy = func(context.Context, hook.Repository) (*model.EffectivePolicy, error) {
		return nil, errUnreadablePolicy
	}
	unbound := sessionDeps(t, root)
	unbound.Session = nil
	for name, deps := range map[string]hook.Dependencies{"unenrolled": unenrolled, "a policy that fails": broken, "no binding": unbound} {
		event, result := hook.Evaluate(t.Context(), "claude", payload, sessionAdapter(), deps)
		if reply := claude.Encode(event, result); reply.ExitCode != 0 || len(reply.Stdout) != 0 || reply.Stderr != "" {
			t.Errorf("%s: reply = %+v", name, reply)
		}
	}
	denied := claude.Encode(model.Event{Shape: model.ShapeSession, Agent: "claude"}, hook.Result{Outcome: hook.Deny, Reason: "x"})
	if denied.ExitCode != 0 || len(denied.Stdout) != 0 {
		t.Errorf("a denial of a session event left: %+v", denied)
	}
}

func TestAnEnvelopeWithoutAToolIsStillRefusedOutsideASession(t *testing.T) {
	t.Parallel()
	if _, err := claude.Parse([]byte(`{"session_id":"s","cwd":"/project","hook_event_name":"PreToolUse"}`)); !errors.Is(err, model.ErrCannotJudge) {
		t.Fatalf("err = %v", err)
	}
}

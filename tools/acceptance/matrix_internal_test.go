package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func simulatedAnswer(want string) result {
	switch want {
	case wantAllow, wantPostAllow:
		return result{}
	case wantDeny:
		return result{stdout: `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"text.dashes: forbidden content (note.txt)"},"systemMessage":"A regra text.dashes encontrou conteúdo proibido em note.txt."}`}
	case wantPost:
		return result{stdout: `{"decision":"block","reason":"text.dashes: forbidden content (note.txt)"}`}
	case wantFailure:
		return result{stdout: `{"hookSpecificOutput":{"hookEventName":"PostToolUseFailure","additionalContext":"text.dashes: forbidden content (note.txt)"}}`}
	default:
		return result{code: 2, stderr: "cannot judge\n"}
	}
}

// fixtureExecutor answers the hook invocation of one fixture with the
// simulated answer its expectation names.
func fixtureExecutor(t *testing.T, base executor, item fixture) executor {
	t.Helper()
	return func(ctx context.Context, c command) result {
		if len(c.args) == 0 || c.args[0] != commandHook {
			return base(ctx, c)
		}
		var envelope struct {
			Event string `json:"hook_event_name"`
		}
		if item.mode != modeRaw {
			if err := json.Unmarshal([]byte(c.input), &envelope); err != nil {
				t.Fatal(err)
			}
		}
		if item.mode == modeShellPost && envelope.Event == eventPre {
			return result{}
		}
		return simulatedAnswer(item.want)
	}
}

func TestMatrixWithSimulatedProcesses(t *testing.T) {
	t.Parallel()
	items := fixtures()
	for i := range items {
		item := items[i]
		t.Run(item.name, func(t *testing.T) {
			t.Parallel()
			s := testSuite(t)
			s.execute = fixtureExecutor(t, s.execute, item)
			assertPass(t, s.runFixture(t.Context(), item))
		})
	}
}

func TestMatrixPayloadErrors(t *testing.T) {
	t.Parallel()
	s := testSuite(t)
	f := writeFixture("broken", pathNote, textSafe, wantAllow)
	if _, err := payload(t.TempDir(), s.home, f); err == nil {
		t.Fatal("missing fixture passed")
	}
	root := t.TempDir()
	path := filepath.Join(root, "tools", "acceptance", "testdata", "claude.json")
	checkedWrite(t, path, "{")
	if _, err := payload(root, s.home, f); err == nil {
		t.Fatal("invalid fixture passed")
	}
	checkedWrite(t, path, "{}")
	f.input = map[string]any{"invalid": make(chan int)}
	if _, err := payload(root, s.home, f); err == nil {
		t.Fatal("unencodable fixture passed")
	}
}

// TestMatrixStopsAtAFailedPreEvent lets the repository setup run and fails
// only the hook, so the shell post fixture reaches its pre event, fails
// there, and never sends the post event it would judge next.
func TestMatrixStopsAtAFailedPreEvent(t *testing.T) {
	t.Parallel()
	s := testSuite(t)
	base := s.execute
	var events []string
	s.execute = func(ctx context.Context, c command) result {
		if len(c.args) == 0 || c.args[0] != commandHook {
			return base(ctx, c)
		}
		var envelope struct {
			Event string `json:"hook_event_name"`
		}
		if err := json.Unmarshal([]byte(c.input), &envelope); err != nil {
			t.Fatal(err)
		}
		events = append(events, envelope.Event)
		return result{code: 2, stderr: "failed"}
	}
	f := fixture{
		name: "failed-pre", mode: modeShellPost, event: eventPost, tool: toolBash,
		input: map[string]any{keyCommand: "true"}, want: wantPost,
	}
	if got := s.runFixture(t.Context(), f); got.status != statusFail || !strings.Contains(got.detail, `unexpected exit 2 or stderr "failed"`) {
		t.Fatalf("failed pre event = %+v", got)
	}
	if !slices.Equal(events, []string{eventPre}) {
		t.Fatalf("hook events = %v, want the pre event only", events)
	}
}

func TestMatrixSetupErrors(t *testing.T) {
	t.Parallel()
	s := testSuite(t)
	repo, err := s.repository(t.Context(), "configuration-errors", false)
	if err != nil {
		t.Fatal(err)
	}
	if s.configureFixture(t.Context(), repo, fixture{mode: modeDeleted}) == nil {
		t.Fatal("missing policy removal passed")
	}
	if s.configureFixture(t.Context(), repo, fixture{mode: modeWeakening}) == nil {
		t.Fatal("missing policy read passed")
	}
	checkedWrite(t, filepath.Join(repo, policyFile), "other: true\n")
	if s.configureFixture(t.Context(), repo, fixture{mode: modeWeakening}) == nil {
		t.Fatal("unrecognized policy passed")
	}
	f := writeFixture("failed", "a", "b", wantAllow)
	s.execute = func(context.Context, command) result { return result{err: ErrAcceptance} }
	if s.runFixture(t.Context(), f).status != statusFail {
		t.Fatal("failed setup passed")
	}
	s = testSuite(t)
	item := aliasRow(t.Context(), s)
	if item.status != statusSkip && item.status != statusFail {
		t.Fatalf("%+v", item)
	}
	if err := os.RemoveAll(s.base); err != nil {
		t.Fatal(err)
	}
}

package cursor_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/cursor"
	"github.com/cristianargotti/golaw/internal/model"
)

func curDocument(tb testing.TB, reply hook.Reply) map[string]string {
	tb.Helper()
	if reply.ExitCode != 0 || len(reply.Stdout) == 0 || reply.Stderr != "" {
		tb.Fatalf("invalid reply transport: %+v", reply)
	}
	var document map[string]string
	if err := json.Unmarshal(reply.Stdout, &document); err != nil {
		tb.Fatal(err)
	}
	return document
}

func TestCursorReplySchemas(t *testing.T) {
	t.Parallel()
	const reason = "Keep hooks enabled."
	const human = "Mantenha os hooks ativados."
	const notice = "The pipeline judges the unmeasured files."
	tests := []struct {
		name   string
		shape  model.EventShape
		result hook.Result
		want   map[string]string
	}{
		{
			"near shell allow", model.ShapeShell,
			hook.Result{Outcome: hook.Allow},
			map[string]string{"permission": "allow"},
		},
		{
			"near read allow", model.ShapeRead,
			hook.Result{Outcome: hook.Allow},
			map[string]string{"permission": "allow"},
		},
		{
			"red shell deny", model.ShapeShell,
			hook.Result{Outcome: hook.Deny, Reason: reason, Human: human},
			map[string]string{"permission": "deny", "agent_message": reason, "user_message": human},
		},
		{
			"red read deny", model.ShapeRead,
			hook.Result{Outcome: hook.Deny, Reason: reason, Human: human},
			map[string]string{"permission": "deny", "user_message": human},
		},
		{
			"red cannot judge", model.ShapeShell,
			hook.Result{Outcome: hook.Deny, CannotJudge: true, Reason: reason, Human: human},
			map[string]string{"permission": "deny", "agent_message": reason, "user_message": human},
		},
		{
			"red read cannot judge", model.ShapeRead,
			hook.Result{Outcome: hook.Deny, CannotJudge: true, Reason: reason, Human: human},
			map[string]string{"permission": "deny", "user_message": human},
		},
		{
			"red post feedback", model.ShapePost,
			hook.Result{Outcome: hook.Feedback, Reason: reason, Human: human},
			map[string]string{"additional_context": reason},
		},
		{
			"red post cannot judge", model.ShapePost,
			hook.Result{Outcome: hook.Feedback, CannotJudge: true, Reason: reason, Human: human},
			map[string]string{"additional_context": reason},
		},
		{
			"near post notice", model.ShapePost,
			hook.Result{Outcome: hook.Allow, Notice: notice},
			map[string]string{"additional_context": notice},
		},
		{
			"near clean post", model.ShapePost,
			hook.Result{Outcome: hook.Allow},
			map[string]string{},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			reply := cursor.Encode(model.Event{Shape: test.shape}, test.result)
			actual := curDocument(t, reply)
			if !reflect.DeepEqual(actual, test.want) {
				t.Fatalf("document = %#v, want %#v", actual, test.want)
			}
		})
	}
}

// TestCursorReplyNeverAllowsWhatTheCoreDidNotAllow holds a result the core
// never produces (an unknown outcome, feedback before a tool runs, a
// denial with blank fields) to a cannot-judge answer in the schema of its
// event, never an allow and never an empty field.
func TestCursorReplyNeverAllowsWhatTheCoreDidNotAllow(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		shape  model.EventShape
		result hook.Result
		field  string
	}{
		{"pre unknown outcome", model.ShapeShell, hook.Result{Outcome: "maybe"}, "agent_message"},
		{"pre feedback before execution", model.ShapeWrite, hook.Result{Outcome: hook.Feedback, Reason: "x"}, "agent_message"},
		{"pre blank denial", model.ShapeShell, hook.Result{Outcome: hook.Deny, Reason: " "}, "agent_message"},
		{"refused envelope without shape", 0, hook.Result{Outcome: "maybe"}, "agent_message"},
		{"read unknown outcome", model.ShapeRead, hook.Result{Outcome: "maybe"}, "user_message"},
		{"post unknown outcome", model.ShapePost, hook.Result{Outcome: "maybe"}, "additional_context"},
		{"post blank feedback", model.ShapePost, hook.Result{Outcome: hook.Feedback}, "additional_context"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			document := curDocument(t, cursor.Encode(model.Event{Shape: test.shape}, test.result))
			if document["permission"] == "allow" || strings.TrimSpace(document[test.field]) == "" {
				t.Fatalf("document = %#v", document)
			}
			if test.shape != model.ShapePost {
				curCheckRefusal(t, document, test.shape == model.ShapeRead)
			}
		})
	}
}

// curCheckRefusal holds a refusal before a tool runs to the denial with a
// line for the person and, except on a read, whose schema has none, the
// cannot-judge reason for the agent.
func curCheckRefusal(t *testing.T, document map[string]string, read bool) {
	t.Helper()
	if document["permission"] != "deny" || strings.TrimSpace(document["user_message"]) == "" {
		t.Fatalf("document = %#v", document)
	}
	agent, present := document["agent_message"]
	if present == read || (present && !strings.HasPrefix(agent, "cannot judge")) {
		t.Fatalf("agent message = %#v", document)
	}
}

// TestCursorPostReplyKeepsTheNoticeOfABlankFinding holds a post result
// whose reason is blank to its notice, the core reason when present and
// otherwise its notice, so the agent reads what the core said.
func TestCursorPostReplyKeepsTheNoticeOfABlankFinding(t *testing.T) {
	t.Parallel()
	const notice = "The pipeline judges the unmeasured files."
	result := hook.Result{Outcome: hook.Feedback, Notice: notice}
	document := curDocument(t, cursor.Encode(model.Event{Shape: model.ShapePost}, result))
	if !reflect.DeepEqual(document, map[string]string{"additional_context": notice}) {
		t.Fatalf("document = %#v", document)
	}
}

package cursor_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/cursor"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestEveryEventIsDeniedUntilTheAdapterIsBuilt(t *testing.T) {
	t.Parallel()
	if _, err := cursor.Parse([]byte("{}")); !errors.Is(err, model.ErrCannotJudge) {
		t.Fatalf("parse = %v", err)
	}
	if _, err := cursor.Oversized(bytes.NewReader([]byte("{}"))); !errors.Is(err, model.ErrCannotJudge) {
		t.Fatalf("oversized = %v", err)
	}
	result := hook.Result{Outcome: hook.Deny, CannotJudge: true, Reason: "cannot judge: x", Human: "não julgado"}
	reply := cursor.Encode(model.Event{}, result)
	var answer struct {
		Permission   string `json:"permission"`
		UserMessage  string `json:"user_message"`
		AgentMessage string `json:"agent_message"`
	}
	if err := json.Unmarshal(reply.Stdout, &answer); err != nil || reply.ExitCode != 0 {
		t.Fatalf("%+v, %v", reply, err)
	}
	if answer.Permission != "deny" || answer.AgentMessage != result.Reason || answer.UserMessage != result.Human {
		t.Fatalf("%+v", answer)
	}
}

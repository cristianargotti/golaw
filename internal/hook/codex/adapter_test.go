package codex_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/codex"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestEveryEventCannotBeJudgedUntilTheAdapterIsBuilt(t *testing.T) {
	t.Parallel()
	if _, err := codex.Parse([]byte("{}")); !errors.Is(err, model.ErrCannotJudge) {
		t.Fatalf("parse = %v", err)
	}
	if _, err := codex.Oversized(bytes.NewReader([]byte("{}"))); !errors.Is(err, model.ErrCannotJudge) {
		t.Fatalf("oversized = %v", err)
	}
	for _, result := range []hook.Result{
		{Outcome: hook.Deny, CannotJudge: true, Reason: "cannot judge: the codex adapter is not built yet"},
		{Outcome: hook.Allow},
	} {
		reply := codex.Encode(model.Event{}, result)
		if reply.ExitCode != 2 || reply.Stderr == "" || reply.Stderr[len(reply.Stderr)-1] != '\n' {
			t.Fatalf("%+v answered %+v", result, reply)
		}
	}
	if codex.PatchGrammar != "0.163.0" {
		t.Fatalf("grammar pin = %s", codex.PatchGrammar)
	}
}

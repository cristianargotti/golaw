package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func completeExecutor(t *testing.T) executor {
	t.Helper()
	base := simulatedExecutor(t, testRoot(t))
	wants := make(map[string]string)
	items := fixtures()
	for i := range items {
		wants[items[i].name] = items[i].want
	}
	wants[caseAlias] = wantDeny
	wants[caseDoctor], wants[caseRecovery] = wantAllow, wantAllow
	return existingAnswers(doctorAnswers(faultExecutor(t, func(ctx context.Context, c command) result {
		if len(c.args) == 0 || c.args[0] != commandHook {
			return base(ctx, c)
		}
		var envelope struct {
			ID    string `json:"tool_use_id"`
			Event string `json:"hook_event_name"`
		}
		if err := json.Unmarshal([]byte(c.input), &envelope); err != nil {
			return simulatedAnswer(wantError)
		}
		want, found := wants[envelope.ID]
		if !found {
			return simulatedAnswer(wantError)
		}
		if envelope.Event == eventPre && strings.HasPrefix(envelope.ID, "post-") ||
			envelope.Event == eventPre && strings.HasPrefix(envelope.ID, "failure-") {
			want = wantAllow
		}
		return simulatedAnswer(want)
	}), reportAnswer(t, nil)), refusalBeforeCommit(), result{})
}

func TestRunInProcess(t *testing.T) {
	t.Parallel()
	lookup := func(name string) (string, error) { return name, nil }
	var out bytes.Buffer
	if code := runArgs(&out, completeExecutor(t), lookup, nil); code != 0 {
		t.Fatalf("%d\n%s", code, out.String())
	}
	if !strings.Contains(out.String(), "\tPASS\t") || !strings.Contains(out.String(), "\tSKIP\t") {
		t.Fatal(out.String())
	}
	out.Reset()
	if code := runArgs(&out, completeExecutor(t), lookup, []string{"-release"}); code != 1 {
		t.Fatalf("%d\n%s", code, out.String())
	}
	if !strings.Contains(out.String(), "host-timeout") || !strings.Contains(out.String(), "host-missing-binary") {
		t.Fatal(out.String())
	}
	out.Reset()
	if runArgs(&out, completeExecutor(t), lookup, []string{"-unknown"}) != 1 {
		t.Fatal("unknown flag passed")
	}
	if runArgs(&out, completeExecutor(t), lookup, []string{"unexpected"}) != 1 {
		t.Fatal("positional argument passed")
	}
	if runArgs(&out, func(context.Context, command) result { return result{err: ErrAcceptance} }, lookup, nil) != 1 {
		t.Fatal("failed build passed")
	}
	if run(failingWriter{}, completeExecutor(t), lookup) != 1 {
		t.Fatal("test flags or failed writer passed")
	}
}

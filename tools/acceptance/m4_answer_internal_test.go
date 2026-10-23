package main

import (
	"strings"
	"testing"
)

// The reason and the human line of the answers below.
const (
	sampleReason = "text.dashes: the text holds a typographic dash (note.txt:1)"
	sampleHuman  = "golaw: o texto tem um travessao tipografico"
)

// codexDeny is the pre denial of the contract: exit 0, the deny JSON on
// stdout and the reason on stderr.
func codexDeny() result {
	return result{
		code:   0,
		stderr: sampleReason + "\n",
		stdout: `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"` +
			sampleReason + `"},"systemMessage":"` + sampleHuman + `"}` + "\n",
	}
}

func TestJudgeCodexHoldsTheAnswersOfTheContract(t *testing.T) {
	t.Parallel()
	notice := `{"hookSpecificOutput":{"hookEventName":"%s","additionalContext":"big.bin was not measured"},"systemMessage":"` + sampleHuman + `"}` + "\n"
	block := `{"decision":"block","reason":"` + sampleReason + `","systemMessage":"` + sampleHuman + `"}` + "\n"
	for _, tc := range []struct {
		name, event string
		want        m4Want
		got         result
		fail        string
	}{
		{"near a pre allow", eventPre, m4Allow, result{}, ""},
		{"near a pre allow with a notice", eventPre, m4Allow, result{stdout: strings.Replace(notice, "%s", eventPre, 1)}, ""},
		{"near a pre denial", eventPre, m4Deny, codexDeny(), ""},
		{"near a post allow", eventPost, m4Allow, result{}, ""},
		{"near a post notice", eventPost, m4Allow, result{stdout: strings.Replace(notice, "%s", eventPost, 1)}, ""},
		{"near a post finding", eventPost, m4Block, result{stdout: block}, ""},
		{"near a post cannot-judge", eventPost, m4Block, result{code: 2, stderr: "cannot judge\n"}, ""},
		{"red the allow JSON Codex records as a failed hook", eventPre, m4Allow, result{
			stdout: `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"allow"}}` + "\n",
		}, "not the notice"},
		{"red a notice of the other event", eventPre, m4Allow, result{stdout: strings.Replace(notice, "%s", eventPost, 1)}, "not the notice"},
		{"red the deny JSON with an empty stderr", eventPre, m4Deny, result{stdout: codexDeny().stdout}, "one reason line"},
		{"red the deny JSON with exit 2, which powershell -Command turns into 1", eventPre, m4Deny, result{code: 2, stderr: codexDeny().stderr, stdout: codexDeny().stdout}, "want exit 0"},
		{"red a stderr reason the JSON does not repeat", eventPre, m4Deny, result{stderr: "other\n", stdout: codexDeny().stdout}, "beside the reason"},
		{"red a field outside the schema", eventPre, m4Deny, result{
			stderr: codexDeny().stderr,
			stdout: strings.Replace(codexDeny().stdout, `"systemMessage"`, `"continue":true,"systemMessage"`, 1),
		}, "unknown field"},
		{"red an indented answer", eventPre, m4Deny, result{
			stderr: codexDeny().stderr,
			stdout: strings.Replace(codexDeny().stdout, `{"hook`, "{ \"hook", 1),
		}, "compact JSON line"},
		{"red no line feed", eventPost, m4Block, result{stdout: strings.TrimSuffix(block, "\n")}, "compact JSON line"},
		{"red a post finding on stderr with exit 0", eventPost, m4Block, result{stderr: "x\n"}, "exited 0"},
		{"red a post cannot-judge that prints JSON", eventPost, m4Block, result{code: 2, stderr: "x\n", stdout: block}, "on stdout"},
		{"red a block without a reason", eventPost, m4Block, result{stdout: `{"decision":"block","systemMessage":"x"}` + "\n"}, "post finding printed"},
		{"red a pre event blocked as post", eventPre, m4Block, codexDeny(), "no Codex answer"},
		{"red a process that never ran", eventPre, m4Allow, result{err: ErrAcceptance}, "run the installed command"},
	} {
		answer, err := judgeCodex(tc.event, tc.want, tc.got)
		if tc.fail == "" && err != nil || tc.fail != "" && (err == nil || !strings.Contains(err.Error(), tc.fail)) {
			t.Errorf("%s: %+v, %v", tc.name, answer, err)
		}
	}
	if answer, err := judgeCodex(eventPre, m4Deny, codexDeny()); err != nil || answer.Reason != sampleReason || answer.Human != sampleHuman {
		t.Fatalf("the denial read back = %+v, %v", answer, err)
	}
}

func TestJudgeCursorHoldsTheAnswersOfD5(t *testing.T) {
	t.Parallel()
	deny := `{"permission":"deny","user_message":"` + sampleHuman + `","agent_message":"` + sampleReason + `"}` + "\n"
	for _, tc := range []struct {
		name, event string
		want        m4Want
		got         result
		fail        string
	}{
		{"near a pre allow", "preToolUse", m4Allow, result{stdout: `{"permission":"allow"}` + "\n"}, ""},
		{"near a shell denial", "beforeShellExecution", m4Deny, result{stdout: deny}, ""},
		{"near a read denial without an agent message", "beforeReadFile", m4Deny, result{stdout: `{"permission":"deny","user_message":"x"}` + "\n"}, ""},
		{"near a quiet post event", "postToolUse", m4Allow, result{stdout: "{}\n"}, ""},
		{"near a post finding", "postToolUseFailure", m4Block, result{stdout: `{"additional_context":"` + sampleReason + `"}` + "\n"}, ""},
		{"red no output, which blocks under failClosed", "preToolUse", m4Allow, result{}, "decode the answer"},
		{"red a read denial with an agent message", "beforeReadFile", m4Deny, result{stdout: deny}, "want"},
		{"red an allow with an extra field", "beforeReadFile", m4Allow, result{stdout: `{"permission":"allow","agent_message":"x"}` + "\n"}, "want"},
		{"red exit 2", "preToolUse", m4Deny, result{code: 2, stdout: deny}, "want exit 0"},
		{"red a denial as allow", "preToolUse", m4Allow, result{stdout: `{"permission":"deny"}` + "\n"}, "permission is"},
		{"red the human line repeating the reason", "preToolUse", m4Deny, result{
			stdout: `{"permission":"deny","user_message":"r","agent_message":"r"}` + "\n",
		}, "repeats"},
		{"red an empty context", "postToolUse", m4Block, result{stdout: `{"additional_context":""}` + "\n"}, "is empty"},
		{"red two documents", "postToolUse", m4Allow, result{stdout: "{}{}\n"}, "one JSON object"},
		{"red a post event denied", "postToolUse", m4Deny, result{stdout: deny}, "no Cursor answer"},
		{"red a process that never ran", "preToolUse", m4Allow, result{err: ErrAcceptance}, "run the installed command"},
	} {
		answer, err := judgeCursor(tc.event, tc.want, tc.got)
		if tc.fail == "" && err != nil || tc.fail != "" && (err == nil || !strings.Contains(err.Error(), tc.fail)) {
			t.Errorf("%s: %+v, %v", tc.name, answer, err)
		}
	}
	if answer, err := judgeCursor("preToolUse", m4Deny, result{stdout: deny}); err != nil || answer.Reason != sampleReason || answer.Human != sampleHuman {
		t.Fatalf("the denial read back = %+v, %v", answer, err)
	}
	if answer, err := judgeCursor("postToolUse", m4Block, result{stdout: `{"additional_context":"` + sampleReason + `"}` + "\n"}); err != nil || answer.Reason != sampleReason {
		t.Fatalf("the post finding read back = %+v, %v", answer, err)
	}
}

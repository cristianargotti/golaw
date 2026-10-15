package main

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"
)

// resultCase is one process result and whether it satisfies an expectation.
type resultCase struct {
	got  result
	want string
	ok   bool
}

func resultCases() []resultCase {
	return []resultCase{
		{result{}, wantAllow, true},
		{result{}, wantPostAllow, true},
		{result{stdout: "{}"}, wantAllow, false},
		{result{stdout: "{}"}, wantPostAllow, false},
		{result{stdout: " "}, wantAllow, false},
		{result{stdout: "{"}, wantDeny, false},
		{result{stdout: `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"Forbidden content"},"systemMessage":"Conteúdo proibido."}`}, wantDeny, true},
		{result{stdout: `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"same"},"systemMessage":"same"}`}, wantDeny, false},
		{result{code: 2, stderr: "cannot judge\n"}, wantError, true},
		{result{code: 2}, wantError, false},
		{result{code: 2, stdout: "{}", stderr: "cannot judge"}, wantError, false},
		{result{stdout: `{"decision":"block","reason":"Forbidden content"}`}, wantPost, true},
		{result{stdout: `{"hookSpecificOutput":{"hookEventName":"PostToolUseFailure","additionalContext":"Forbidden content"}}`}, wantFailure, true},
		{result{stdout: "{}"}, "unknown", false},
		{result{stderr: "unexpected"}, wantAllow, false},
		{result{code: 1}, wantAllow, false},
		{result{err: ErrAcceptance}, wantAllow, false},
	}
}

func TestAcceptanceResults(t *testing.T) {
	t.Parallel()
	for _, tc := range resultCases() {
		if err := expect(tc.got, tc.want); (err == nil) != tc.ok {
			t.Fatalf("%+v: %v", tc, err)
		}
	}
	for _, decision := range []string{"allow", "ask", "defer", "deny"} {
		body := `{"hookSpecificOutput":{"permissionDecision":"` + decision + `"}}`
		for _, want := range []string{wantAllow, wantPostAllow} {
			if validAnswer(result{stdout: body}, want) {
				t.Fatalf("%s accepted %s", want, body)
			}
		}
	}
	if success(result{}) != nil || success(result{code: 1}) == nil ||
		!errors.Is(success(result{err: ErrAcceptance}), ErrAcceptance) {
		t.Fatal("unexpected process success assertion")
	}
}

func TestRunCommand(t *testing.T) {
	t.Parallel()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	base := command{
		program: binary, argv0: "acceptance-child",
		args: []string{"-test.run=^TestAcceptanceChild$"},
		dir:  t.TempDir(), env: []string{"GOLAW_ACCEPTANCE_CHILD=1", "GOCOVERDIR=" + t.TempDir()},
		input: "{}",
	}
	got := runCommand(t.Context(), base)
	if got.err != nil || got.code != 2 || got.stdout != "{}" || got.stderr != "denied" {
		t.Fatalf("%+v", got)
	}
	base.env = []string{"GOLAW_ACCEPTANCE_CHILD=0", "GOCOVERDIR=" + t.TempDir()}
	if got := runCommand(t.Context(), base); got.err != nil || got.code != 0 {
		t.Fatalf("%+v", got)
	}
	base.program = "missing-golaw-acceptance-executable"
	if runCommand(t.Context(), base).err == nil {
		t.Fatal("missing executable succeeded")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	base.program = binary
	if !errors.Is(runCommand(ctx, base).err, context.Canceled) {
		t.Fatal("cancellation was not reported")
	}
}

func TestAcceptanceChild(t *testing.T) {
	t.Parallel()
	if os.Getenv("GOLAW_ACCEPTANCE_CHILD") != "1" {
		return
	}
	body, err := io.ReadAll(os.Stdin)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stdout.Write(body); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stderr.WriteString("denied"); err != nil {
		t.Fatal(err)
	}
	os.Exit(2) //nolint:revive // the child process stands in for a denying hook and must exit 2
}

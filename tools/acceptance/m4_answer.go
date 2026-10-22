package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// decisionBlock is the decision of a Codex post finding.
const decisionBlock = "block"

// m4Want is what a replayed payload must answer.
type m4Want int

// The three answers a row asks for: the call runs, with no answer or a
// notice the agent reads; a pre event is refused, a cannot-judge answer
// included; a post event hands the agent a finding.
const (
	m4Allow m4Want = iota + 1
	m4Deny
	m4Block
)

// m4Answer is what the harness reads back from one reply: the English
// reason, the human line and the notice, each empty when the answer has
// none.
type m4Answer struct {
	Reason, Human, Notice string
}

// codexHook is the hookSpecificOutput of a Codex answer.
type codexHook struct {
	Event      string `json:"hookEventName"`
	Decision   string `json:"permissionDecision"`
	Reason     string `json:"permissionDecisionReason"`
	Additional string `json:"additionalContext"`
}

// codexDocument is every field a Codex answer of golaw may carry (section
// 2.3 of the contract): a field outside them makes Codex reject the answer
// and run the tool (survey section 3.1), so decoding refuses it.
type codexDocument struct {
	Hook     *codexHook `json:"hookSpecificOutput"`
	Decision string     `json:"decision"`
	Reason   string     `json:"reason"`
	Human    string     `json:"systemMessage"`
}

// judgeCodex holds a reply of `golaw hook codex` to the answer of its
// event the contract fixes byte for byte: stdout, stderr and exit code.
func judgeCodex(event string, want m4Want, got result) (m4Answer, error) {
	if got.err != nil {
		return m4Answer{}, fmt.Errorf("run the installed command: %w", got.err)
	}
	post := event == eventPost
	switch {
	case want == m4Allow:
		return codexQuiet(event, got)
	case want == m4Deny && !post:
		return codexDenied(got)
	case want == m4Block && post:
		return codexBlocked(got)
	}
	return m4Answer{}, fmt.Errorf("no Codex answer %d exists for %s: %w", want, event, ErrAcceptance)
}

// codexQuiet is an allow: exit 0, no stderr, and no output or the notice
// of the event, which Codex hands the model while the tool runs.
func codexQuiet(event string, got result) (m4Answer, error) {
	if got.code != 0 || got.stderr != "" {
		return m4Answer{}, fmt.Errorf("an allow exited %d with stderr %q: %w", got.code, got.stderr, ErrAcceptance)
	}
	if got.stdout == "" {
		return m4Answer{}, nil
	}
	var answer codexDocument
	if err := strictLine(got.stdout, &answer); err != nil {
		return m4Answer{}, err
	}
	if answer.Hook == nil || answer.Hook.Event != event || answer.Hook.Additional == "" || answer.Hook.Decision != "" ||
		answer.Hook.Reason != "" || answer.Human == "" || answer.Decision != "" || answer.Reason != "" {
		return m4Answer{}, fmt.Errorf("an allow printed %q, not the notice of %s: %w", got.stdout, event, ErrAcceptance)
	}
	return m4Answer{Human: answer.Human, Notice: answer.Hook.Additional}, nil
}

// codexDenied is a pre denial: exit 0 with the deny JSON on stdout, the
// answer Codex reads through every shell, and the same reason on stderr
// for the person.
func codexDenied(got result) (m4Answer, error) {
	reason, err := codexStderr(got, 0)
	if err != nil {
		return m4Answer{}, err
	}
	var answer codexDocument
	if err := strictLine(got.stdout, &answer); err != nil {
		return m4Answer{}, err
	}
	if answer.Hook == nil || answer.Hook.Event != eventPre || answer.Hook.Decision != wantDeny ||
		answer.Hook.Reason != reason || answer.Hook.Additional != "" || answer.Decision != "" || answer.Reason != "" ||
		answer.Human == "" || answer.Human == reason {
		return m4Answer{}, fmt.Errorf("a denial printed %q beside the reason %q: %w", got.stdout, reason, ErrAcceptance)
	}
	return m4Answer{Reason: reason, Human: answer.Human}, nil
}

// codexBlocked is a post finding, the block JSON with exit 0, or a post
// cannot-judge, exit 2 with the reason on stderr and nothing on stdout.
func codexBlocked(got result) (m4Answer, error) {
	if got.code == 2 {
		reason, err := codexStderr(got, 2)
		if err == nil && got.stdout != "" {
			err = fmt.Errorf("a post cannot-judge printed %q on stdout: %w", got.stdout, ErrAcceptance)
		}
		return m4Answer{Reason: reason}, err
	}
	if got.code != 0 || got.stderr != "" {
		return m4Answer{}, fmt.Errorf("a post finding exited %d with stderr %q: %w", got.code, got.stderr, ErrAcceptance)
	}
	var answer codexDocument
	if err := strictLine(got.stdout, &answer); err != nil {
		return m4Answer{}, err
	}
	if answer.Hook != nil || answer.Decision != decisionBlock || answer.Reason == "" || answer.Human == "" {
		return m4Answer{}, fmt.Errorf("a post finding printed %q: %w", got.stdout, ErrAcceptance)
	}
	return m4Answer{Reason: answer.Reason, Human: answer.Human}, nil
}

// codexStderr is the reason line of an answer with the exit code: one
// non-empty line on stderr, which a denial repeats for the person and
// which is all Codex reads on an exit 2, failing open when it is empty.
func codexStderr(got result, code int) (string, error) {
	reason, ended := strings.CutSuffix(got.stderr, "\n")
	if got.code != code || !ended || strings.TrimSpace(reason) == "" || strings.Contains(reason, "\n") {
		return "", fmt.Errorf("exit %d with stderr %q, want exit %d and one reason line: %w", got.code, got.stderr, code, ErrAcceptance)
	}
	return reason, nil
}

// strictLine decodes stdout as one compact JSON document ending with a
// line feed, refusing a field the target does not name.
func strictLine(stdout string, target any) error {
	body, ended := strings.CutSuffix(stdout, "\n")
	var compact bytes.Buffer
	if !ended || json.Compact(&compact, []byte(body)) != nil || compact.String() != body {
		return fmt.Errorf("stdout %q is not one compact JSON line: %w", stdout, ErrAcceptance)
	}
	decoder := json.NewDecoder(strings.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode the answer %q: %w", stdout, err)
	}
	if decoder.More() {
		return fmt.Errorf("stdout %q holds a second document: %w", stdout, ErrAcceptance)
	}
	return nil
}

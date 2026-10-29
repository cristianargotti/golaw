package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// m6DenialSamples are reasons a candidate prints for the red write of each
// row, the M6.6.5 sentence with the path and line of the fixtures.
func m6DenialSamples() map[string]string {
	return map[string]string{
		"code-eval":            "test/view/eval.test.cjs:2 runs text as code with new Function (Sonar S1523): export the code from a module and call it; writing it to a file and requiring it is the same execution.",
		"hardcoded-credential": "test/deploy/test_login.py:2 assigns a literal to " + m6CredentialWord + " (Sonar S2068): generate it at run time (secrets.token_urlsafe(16), crypto.randomUUID()) or read it from the environment.",
		"file-length":          "scripts/long.sh has 301 lines, the limit is 300: split it by responsibility.",
		"inline-script-size":   "views/report.html:7 holds an inline script of 61 lines, the limit is 40: move it to views/report.js, embed it with go:embed and splice it when the view is served, so the linters read it.",
		"suppression":          "views/common.js:2 adds " + m6SuppressionWord + ", which hides every rule on the line from the pipeline's analyzer: fix the finding; golaw lint names it.",
		"analyzer-widening":    "sonar-project.properties adds src/legacy/** to sonar.exclusions: an exclusion hides code from the pipeline's gate; fix the findings, or ask the person to change the scope.",
		"tag-and-digest":       "deploy/cache.Dockerfile:1 writes FROM with a tag and a digest (Sonar S8431): keep only the digest and put the tag in a comment on the line above.",
	}
}

func m6JSON(t *testing.T, value any) string {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(body) + "\n"
}

// m6DenialReplies stand in for a candidate whose three agents deny each red
// write and allow each near miss: Claude and Codex answer before the write,
// Cursor allows the captured write and judges it after it ran.
func m6DenialReplies(t *testing.T, reason, codexReason string) []m5Reply {
	t.Helper()
	claude, codex, cursor := 0, 0, 0
	deny := func(r string) map[string]any {
		return map[string]any{"systemMessage": "A operação foi bloqueada.", "hookSpecificOutput": map[string]string{
			"hookEventName": eventPre, "permissionDecision": wantDeny, "permissionDecisionReason": r,
		}}
	}
	return []m5Reply{
		m6InitReply(),
		{prefix: []string{commandHook, agentClaude}, answer: func(t *testing.T, _ command) result {
			t.Helper()
			claude++
			if claude%2 == 1 {
				return result{stdout: m6JSON(t, deny(reason))}
			}
			return result{}
		}},
		{prefix: []string{commandHook, agentCodex}, answer: func(t *testing.T, _ command) result {
			t.Helper()
			codex++
			if codex%2 == 1 {
				return result{stdout: m6JSON(t, deny(codexReason)), stderr: codexReason + "\n"}
			}
			return result{}
		}},
		{prefix: []string{commandHook, agentCursor}, answer: func(t *testing.T, _ command) result {
			t.Helper()
			cursor++
			switch cursor % 4 {
			case 1, 3:
				return result{stdout: `{"permission":"allow"}` + "\n"}
			case 2:
				return result{stdout: m6JSON(t, map[string]string{"additional_context": reason})}
			default:
				return result{stdout: "{}\n"}
			}
		}},
	}
}

func TestM6DenialReasonsAreTheSentencesOfTheAddendum(t *testing.T) {
	t.Parallel()
	samples := m6DenialSamples()
	if len(m6Denials()) != len(samples) {
		t.Fatalf("%d denials, %d samples", len(m6Denials()), len(samples))
	}
	for _, d := range m6Denials() {
		if !d.reason.MatchString(samples[d.name]) || d.reason.MatchString(samples[d.name]+" ") {
			t.Errorf("%s: %s does not hold the sample exactly", d.name, d.reason)
		}
		red, near, err := d.contents(testRoot(t))
		if err != nil || len(red) == 0 || len(near) == 0 || bytes.Equal(red, near) {
			t.Errorf("%s: red %d bytes, near %d bytes, %v", d.name, len(red), len(near), err)
		}
	}
	if got := bytes.Count(m6LongScript(true), []byte("\n")); got != 301 {
		t.Errorf("the long script holds %d lines, want 301", got)
	}
	if got := bytes.Count(m6LongScript(false), []byte("\n")); got != 300 {
		t.Errorf("the near script holds %d lines, want 300", got)
	}
}

func TestM6DenialRowsHoldTheReasonAcrossTheAgents(t *testing.T) {
	t.Parallel()
	samples := m6DenialSamples()
	for _, d := range m6Denials() {
		for _, tc := range []struct {
			kind          string
			reason, codex string
			pass          bool
		}{
			{"near the same reason everywhere", samples[d.name], samples[d.name], true},
			{"red Codex prints another reason", samples[d.name], samples[d.name] + " Again.", false},
			{"red the reason of another sentence", "denied", "denied", false},
		} {
			s := m6TestSuite(t)
			s.execute = m5Script(t, s.suite, m6DenialReplies(t, tc.reason, tc.codex)...).Execute
			err := s.m6Scope(m6DenialCategory, d.name).m6Deny(t.Context(), d)
			if (err == nil) != tc.pass {
				t.Errorf("%s over %s: %v", d.name, tc.kind, err)
			}
		}
	}
}

func TestM6CorrectedTwinRaisesNoDenial(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		deny bool
	}{
		{"near every write allowed", false},
		{"red one write denied", true},
	} {
		s := m6TestSuite(t)
		calls := 0
		s.execute = m5Script(t, s.suite, m6InitReply(), m5Reply{prefix: []string{commandHook, agentClaude}, answer: func(t *testing.T, c command) result {
			t.Helper()
			calls++
			if tc.deny && strings.Contains(c.input, "panel.js") {
				return result{stdout: m6JSON(t, map[string]any{"systemMessage": "x", "hookSpecificOutput": map[string]string{
					"hookEventName": eventPre, "permissionDecision": wantDeny, "permissionDecisionReason": "denied",
				}})}
			}
			return result{}
		}}).Execute
		err := s.m6Scope(m6DenialCategory, "corrected-twin").m6CorrectedTwin(t.Context())
		if (err != nil) != tc.deny || calls == 0 {
			t.Errorf("%s: %v after %d writes", tc.name, err, calls)
		}
	}
}

func TestM6PatchAddsANewFileAndRewritesAnExistingOne(t *testing.T) {
	t.Parallel()
	added := m6Patch("views/a.js", nil, []byte("one\ntwo\n"), false)
	if added != "*** Begin Patch\n*** Add File: views/a.js\n+one\n+two\n*** End Patch" {
		t.Fatalf("add = %q", added)
	}
	updated := m6Patch("a.properties", []byte("k=1\n"), []byte("k=2\n"), true)
	if updated != "*** Begin Patch\n*** Update File: a.properties\n@@\n-k=1\n+k=2\n*** End Patch" {
		t.Fatalf("update = %q", updated)
	}
}

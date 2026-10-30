package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/surfacefix"
)

func TestM6FixtureDecodesTheEscapedFilesOnly(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, filepath.FromSlash(m6FixtureRoot), "sample")
	checkedWrite(t, filepath.Join(dir, "views", "a.js.esc"), "const a = \"\\u0041\";\n")
	got, err := m6Fixture(root, "sample")
	if err != nil || string(got["views/a.js"]) != "const a = \"A\";\n" || len(got) != 1 {
		t.Fatalf("near the escaped file = %q, %v", got, err)
	}
	checkedWrite(t, filepath.Join(dir, "plain.js"), "x\n")
	if _, err := m6Fixture(root, "sample"); err == nil || !strings.Contains(err.Error(), "plain.js") {
		t.Fatalf("red a file without the suffix = %v", err)
	}
	if _, err := m6Fixture(root, "absent"); err == nil {
		t.Fatal("red an absent fixture directory was read")
	}
}

func TestM6WithoutFeedbackEditsTheLadderLine(t *testing.T) {
	t.Parallel()
	got, err := m6WithoutFeedback([]byte(m6PolicySample))
	want := "ladder: { pre_commit: true, commit_msg: true, pre_push: true, test_staged: false, hook_feedback: false }\n"
	if err != nil || !bytes.HasSuffix(got, []byte(want)) {
		t.Fatalf("near the rendered ladder = %q, %v", got, err)
	}
	if _, err := m6WithoutFeedback([]byte("golaw: 1\n")); err == nil {
		t.Fatal("red a policy without the ladder line was edited")
	}
}

func TestM6TwinIsAdoptedAndCommitted(t *testing.T) {
	t.Parallel()
	s := m6TestSuite(t)
	s.execute = m5Script(t, s.suite, m6InitReply()).Execute
	repo, err := s.m6Twin(t.Context(), "twin", surfacefix.Corrected, m6WithoutFeedback)
	if err != nil {
		t.Fatal(err)
	}
	m5Clean(t, s.suite, repo)
	policy, err := readFile(filepath.Join(repo, policyFile))
	if err != nil || !bytes.Contains(policy, []byte("hook_feedback: false")) {
		t.Fatalf("policy = %q, %v", policy, err)
	}
	panel, err := readFile(filepath.Join(repo, "views", "panel.js"))
	if err != nil || !bytes.Contains(panel, []byte("export function label")) {
		t.Fatalf("the corrected twin is not in the checkout: %q, %v", panel, err)
	}
}

func TestM6ClaudeSendsTheWriteOfThePath(t *testing.T) {
	t.Parallel()
	s := m6TestSuite(t)
	var input string
	s.execute = m5Script(t, s.suite, m5Reply{prefix: []string{commandHook, agentClaude}, answer: func(_ *testing.T, c command) result {
		input = c.input
		return result{}
	}}).Execute
	repo := t.TempDir()
	if _, err := s.m6Claude(t.Context(), repo, eventPost, "scripts/a.sh", []byte("#!/bin/sh\n"), s.env); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"hook_event_name":"PostToolUse"`, `"tool_name":"Write"`, `a.sh"`, `"content":"#!/bin/sh\n"`} {
		if !strings.Contains(input, want) {
			t.Fatalf("payload %s lacks %s", input, want)
		}
	}
}

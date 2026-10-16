package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInvocationValidation(t *testing.T) {
	t.Parallel()
	s := testSuite(t)
	if _, err := s.handler("unknown", toolWrite); err == nil {
		t.Fatal("missing event passed")
	}
	s.hooks[eventPre] = []hookGroup{{Matcher: "[", Hooks: nil}}
	if _, err := s.handler(eventPre, toolWrite); err == nil {
		t.Fatal("invalid matcher passed")
	}
	s.hooks[eventPre] = []hookGroup{{
		Matcher: toolWrite, Hooks: []hookHandler{
			{Command: "foreign"},
			{Type: keyCommand, Command: binaryName, Args: []string{commandHook, agentClaude}, Timeout: 1},
		},
	}}
	if _, err := s.handler(eventPre, toolWrite); err == nil {
		t.Fatal("invalid exec form passed")
	}
	if s.invoke(t.Context(), s.home, eventPre, toolWrite, nil).err == nil {
		t.Fatal("invalid invocation passed")
	}
	s = testSuite(t)
	if err := os.Remove(s.binary); err != nil {
		t.Fatal(err)
	}
	if s.invoke(t.Context(), s.home, eventPre, toolWrite, nil).err == nil {
		t.Fatal("missing executable passed")
	}
	other := t.TempDir()
	checkedWrite(t, filepath.Join(other, executable(binaryName)), "other")
	s.env = prependPath(s.env, other)
	if s.invoke(t.Context(), s.home, eventPre, toolWrite, nil).err == nil {
		t.Fatal("different executable passed")
	}
}

func TestDiscoverHooks(t *testing.T) {
	t.Parallel()
	s := testSuite(t)
	if _, _, err := discoverHooks(t.TempDir()); err == nil {
		t.Fatal("absent hooks passed")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, hooksFileName)
	checkedWrite(t, path, "{")
	if _, _, err := discoverHooks(dir); err == nil {
		t.Fatal("invalid JSON passed")
	}
	checkedWrite(t, path, `{"hooks":{"PreToolUse":[{"matcher":"Write","hooks":[{"command":"foreign"}]}]}}`)
	if _, _, err := discoverHooks(dir); err == nil {
		t.Fatal("foreign hooks passed")
	}
	body, err := readFile(s.hookFile)
	if err != nil {
		t.Fatal(err)
	}
	checkedWrite(t, path, string(body))
	checkedWrite(t, filepath.Join(dir, "second", hooksFileName), string(body))
	if _, _, err := discoverHooks(dir); err == nil {
		t.Fatal("ambiguous hooks passed")
	}
	if _, _, err := discoverHooks(filepath.Join(dir, "absent")); err == nil {
		t.Fatal("absent home passed")
	}
}

package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/profile"
)

// m5FreshGoverned answers a governed operation as the loader does: the one
// line of D8 when the bundle of the command's home is absent, done
// otherwise.
func m5FreshGoverned(code int) func(*testing.T, command) result {
	return func(t *testing.T, c command) result {
		t.Helper()
		if _, err := os.Stat(profile.Dir(m5TestDirs(t, c.env).Config, m5Name)); err != nil {
			return result{code: code, stderr: "golaw: profile fixture is not installed: run " + m5InstallCommand + "\n"}
		}
		return result{}
	}
}

// m5FreshHook answers the agent hook: the valid allow where no record of
// the machine enrolls the directory (A3), the denial of D8 where one does
// and the bundle is absent.
func m5FreshHook(t *testing.T, c command) result {
	t.Helper()
	dirs := m5TestDirs(t, c.env)
	_, enrolled, err := policy.FindRecord(dirs.State, c.dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, statErr := os.Stat(profile.Dir(dirs.Config, m5Name)); !enrolled || statErr == nil {
		return result{}
	}
	body, err := json.Marshal(map[string]any{
		"systemMessage": "Perfil fixture ausente ou divergente: execute " + m5InstallCommand,
		"hookSpecificOutput": map[string]any{
			"hookEventName": eventPre, "permissionDecision": wantDeny,
			"permissionDecisionReason": "profile fixture is not installed: run " + m5InstallCommand,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return result{stdout: string(body)}
}

// m5FreshReplies are the answers of the fresh machine rows.
func m5FreshReplies(t *testing.T) []m5Reply {
	t.Helper()
	initAnswer := func(t *testing.T, c command) result {
		t.Helper()
		if _, err := os.Stat(filepath.Join(c.dir, policyFile)); err == nil {
			return m5FreshGoverned(2)(t, c)
		}
		m5FakeInit(t, c)
		m5FakeEnroll(t, c)
		return result{}
	}
	return []m5Reply{
		{prefix: []string{m5ProfileCommand, m5InstallSub}, answer: m5LifecycleInstall},
		{prefix: []string{m5ProfileCommand, "remove"}, answer: m5LifecycleRemove},
		{prefix: []string{commandInit}, answer: initAnswer},
		{prefix: []string{commandScan}, answer: m5FreshGoverned(2)},
		{prefix: []string{"agent", m5InstallSub}, answer: m5FreshGoverned(2)},
		{prefix: []string{"setup"}, answer: m5Answer(result{})},
		{prefix: []string{commandHook, "pre-commit"}, answer: m5FreshGoverned(1)},
		{prefix: []string{commandHook, agentClaude}, answer: m5FreshHook},
	}
}

func TestM5FreshRowsWithASimulatedCandidate(t *testing.T) {
	t.Parallel()
	defects := []m5Reply{
		{prefix: []string{commandScan}, answer: m5Answer(result{code: 2})},
		{prefix: []string{commandInit, flagApply}, answer: m5Answer(result{})},
		{prefix: []string{"agent", m5InstallSub}, answer: m5Answer(result{})},
		{prefix: []string{commandHook}, answer: m5Answer(result{code: 2, stderr: "golaw: cannot judge\n"})},
	}
	for _, c := range m5FreshCases() {
		s := m2TestSuite(t)
		scope := s.m5Scope(m5FreshCategory, c.name)
		scope.execute = m5Script(t, scope, m5FreshReplies(t)...).Execute
		if err := c.run(scope, t.Context()); err != nil {
			t.Errorf("%s: %v", c.name, err)
		}
		silent := s.m5Scope(m5FreshCategory, c.name+"-silent")
		silent.execute = m5Script(t, silent, append(slices.Clone(defects), m5FreshReplies(t)...)...).Execute
		if err := c.run(silent, t.Context()); err == nil {
			t.Errorf("%s passed over a candidate that refuses silently or denies a fresh clone", c.name)
		}
	}
	want := []string{"scan", "scan-enrolled", "rung", "hook-enrolled", "init-apply", "agent-install", "unenrolled-allow"}
	if got := m5Names(m5FreshCases()); !slices.Equal(got, want) {
		t.Fatalf("names = %q", got)
	}
}

func TestM5JudgeDenialNamesTheCommandForTheAgentAndThePerson(t *testing.T) {
	t.Parallel()
	deny := func(reason, human string) result {
		body, err := json.Marshal(map[string]any{"systemMessage": human, "hookSpecificOutput": map[string]any{
			"hookEventName": eventPre, "permissionDecision": wantDeny, "permissionDecisionReason": reason,
		}})
		if err != nil {
			t.Fatal(err)
		}
		return result{stdout: string(body)}
	}
	reason := "profile fixture is not installed: run " + m5InstallCommand
	human := "Perfil fixture ausente ou divergente: execute " + m5InstallCommand
	for _, row := range []struct {
		name string
		got  result
		pass bool
	}{
		{"near both channels", deny(reason, human), true},
		{"red no command for the agent", deny("profile fixture is not installed", human), false},
		{"red no line for the person", deny(reason, "Perfil ausente"), false},
		{"red an allow", result{}, false},
	} {
		if err := m5JudgeDenial(row.got); (err == nil) != row.pass {
			t.Errorf("%s: %v", row.name, err)
		}
	}
	repo := t.TempDir()
	checkedWrite(t, filepath.Join(repo, policyFile), "golaw: 1\n")
	if _, err := m5Enrolled(repo); !errors.Is(err, ErrAcceptance) {
		t.Fatalf("a policy without a pin = %v", err)
	}
	pinned := "golaw: 1\nkit: { version: 0.5.0 }\nprofile: { name: fixture, source: s, version: 1.2.3, digest: sha256:" + strings.Repeat("a", 64) + " }\n"
	checkedWrite(t, filepath.Join(repo, policyFile), pinned)
	digests, err := m5Enrolled(repo)
	if err != nil || !strings.HasSuffix(digests, ",sha256:"+strings.Repeat("a", 64)) || len(digests) != 64+1+71 {
		t.Fatalf("digests = %q, %v", digests, err)
	}
}

func TestM5FreshRowsReportEveryFailedStep(t *testing.T) {
	t.Parallel()
	for _, c := range m5FreshCases() {
		m5Sweep(t, m5FreshCategory, c, func(t *testing.T, _ *suite) []m5Reply {
			t.Helper()
			return m5FreshReplies(t)
		})
	}
}

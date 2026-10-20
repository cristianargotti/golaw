package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/rung"
)

func TestM2SwitchesBypassPinsToolsAndInvalidInputs(t *testing.T) {
	t.Parallel()
	for _, rungName := range rung.Names() {
		s, repo := m2PinRepository(t)
		name := rungName + "-disabled"
		script := m2LifecycleScript(t, s, name, "answer")
		s.execute = func(ctx context.Context, c command) result {
			if c.program == s.binary && slices.Contains(c.args, commandHook) {
				m2RequireDisabledCommand(t, c, rungName)
				m2RequireSwitchedPolicy(t, s, c.dir, rungName)
			}
			return script.Execute(ctx, c)
		}
		if err := s.m2SwitchCase(t.Context(), repo, name); err != nil {
			t.Fatal(err)
		}
		if script.Targets != 1 {
			t.Fatalf("disabled rung targets = %d", script.Targets)
		}
	}
}

func m2RequireDisabledCommand(t *testing.T, c command, rungName string) {
	t.Helper()
	for _, name := range []string{"go", "git", binaryName, "sh"} {
		if _, err := resolveExecutable(name, c.env); err != nil {
			t.Fatalf("disabled rung cannot resolve prerequisite %s: %v", name, err)
		}
	}
	if err := m2MissingTools(c.env); err != nil {
		t.Fatalf("disabled rung retained optional tools on PATH: %v", err)
	}
	if rungName == rung.PrePush && c.input != "malformed ref stream\n" {
		t.Fatalf("disabled push input = %q", c.input)
	}
	if rungName == rung.CommitMsg {
		if _, err := os.Stat(c.args[len(c.args)-1]); !os.IsNotExist(err) {
			t.Fatalf("disabled commit message was available: %v", err)
		}
	}
}

func m2RequireSwitchedPolicy(t *testing.T, s *suite, repo, rungName string) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(repo, policyFile))
	if err != nil {
		t.Fatal(err)
	}
	file, err := policy.Decode(body)
	if err != nil || file.Kit.Version != m2OlderVersion || file.Ladder == nil {
		t.Fatalf("checkout pin or ladder = %+v, %v", file, err)
	}
	switches := map[string]*bool{
		rung.PreCommit: file.Ladder.PreCommit, rung.CommitMsg: file.Ladder.CommitMsg, rung.PrePush: file.Ladder.PrePush,
	}
	for name, on := range switches {
		if on == nil || *on != (name != rungName) {
			t.Fatalf("%s: checkout switch = %v", name, on)
		}
	}
	root, _, err := install.ProjectIdentity(repo)
	if err != nil {
		t.Fatal(err)
	}
	record, found, err := policy.ReadRecord(m2Dirs(t, s, s.env).State, root)
	if err != nil || !found || record.KitVersion != m2OlderVersion || record.PolicyDigest != policy.Digest(file) {
		t.Fatalf("disabled enrollment does not match policy: %+v %v", record, err)
	}
	committed, err := policy.Decode([]byte(record.Bootstrap))
	if err != nil || committed.Ladder == nil || committed.Ladder.PreCommit == nil ||
		!*committed.Ladder.PreCommit || committed.Ladder.CommitMsg == nil || !*committed.Ladder.CommitMsg ||
		committed.Ladder.PrePush == nil || !*committed.Ladder.PrePush {
		t.Fatalf("disabled row changed the enrolled ladder: %v", err)
	}
}

func TestM2SwitchesRejectEffectsAndNoisyAnswers(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"checkout", "enrollment", "snapshot", "state", "stdout", "stderr", "exit", "process"} {
		s, repo := m2PinRepository(t)
		script := m2LifecycleScript(t, s, "pre-push-disabled", "answer")
		s.execute = func(ctx context.Context, c command) result {
			got := script.Execute(ctx, c)
			if c.program == s.binary && slices.Contains(c.args, commandHook) {
				got = m2SwitchFault(t, s, c, mode)
			}
			return got
		}
		if err := s.m2SwitchCase(t.Context(), repo, "pre-push-disabled"); err == nil || script.Targets != 1 {
			t.Fatalf("%s: target=%d error=%v", mode, script.Targets, err)
		}
	}
}

func m2SwitchFault(t *testing.T, s *suite, c command, mode string) result {
	t.Helper()
	switch mode {
	case "checkout":
		m2Write(t, filepath.Join(c.dir, pathNote), m2Changed)
	case "enrollment":
		m2Write(t, filepath.Join(m2Dirs(t, s, s.env).State, "unexpected"), m2Changed)
	case "snapshot":
		if err := os.Remove(filepath.Join(c.dir, gitDirectory, "index")); err != nil {
			t.Fatal(err)
		}
	case "state":
		if err := os.RemoveAll(m2Dirs(t, s, s.env).State); err != nil {
			t.Fatal(err)
		}
	case "stdout":
		return result{stdout: "unexpected finding"}
	case "stderr":
		return result{stderr: "unexpected refusal"}
	case "exit":
		return result{code: 1}
	case "process":
		return result{err: ErrAcceptance}
	}
	return result{}
}

func TestM2SwitchesRejectInvalidFixtures(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"missing", "decode", "absent", "already-disabled"} {
		repo := t.TempDir()
		if mode != "missing" {
			body := "golaw: 1\nkit: { version: 0.1.0 }\n"
			if mode == "decode" {
				body = "unknown: true\n"
			}
			if mode == "already-disabled" {
				body += "ladder: { pre_commit: false }\n"
			}
			m2Write(t, filepath.Join(repo, policyFile), body)
		}
		if err := m2DisableRung(repo, rung.PreCommit); err == nil {
			t.Fatalf("%s: invalid switch fixture passed", mode)
		}
	}
	for _, name := range []string{"invalid", "unknown-disabled", "pre-commit-equal"} {
		s := m2TestSuite(t)
		if err := s.m2SwitchCase(t.Context(), t.TempDir(), name); err == nil {
			t.Fatalf("%s: invalid case passed", name)
		}
	}
}

func TestM2PinAndSwitchRequireVerifiedSetup(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"pre-commit-older", "pre-commit-disabled"} {
		s, repo := m2PinRepository(t)
		s.m2.Ready = false
		var err error
		if strings.HasSuffix(name, "-disabled") {
			err = s.m2SwitchCase(t.Context(), repo, name)
		} else {
			err = s.m2PinCase(t.Context(), repo, name)
		}
		if err == nil {
			t.Fatalf("%s: unverified setup passed", name)
		}
	}
}

func TestM2SwitchObservationFailures(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"snapshot", "enrollment", "environment", "prerequisite", "optional-tool"} {
		s, repo := m2PinRepository(t)
		m2InvalidateSwitchObservation(t, s, repo, mode)
		fixture := s.execute
		targeted := false
		s.execute = func(ctx context.Context, c command) result {
			targeted = targeted || c.program == s.binary
			return fixture(ctx, c)
		}
		if err := s.m2SilentRung(t.Context(), repo, rung.PreCommit); err == nil || targeted {
			t.Fatalf("%s: target=%t error=%v", mode, targeted, err)
		}
	}
}

func m2InvalidateSwitchObservation(t *testing.T, s *suite, repo, mode string) {
	t.Helper()
	switch mode {
	case "snapshot":
		if err := os.Remove(filepath.Join(repo, gitDirectory, "index")); err != nil {
			t.Fatal(err)
		}
	case "enrollment":
		if err := os.RemoveAll(m2Dirs(t, s, s.env).State); err != nil {
			t.Fatal(err)
		}
	case "environment":
		m2InvalidatePin(t, s, repo, "dirs")
	case "prerequisite":
		s.lookup = func(string) (string, error) { return "", ErrAcceptance }
	case "optional-tool":
		dir := filepath.Join(s.base, "prerequisites")
		if s.m2OS == goosWindows {
			dir = filepath.Dir(s.binary)
		}
		m2Write(t, filepath.Join(dir, executable("gitleaks")), "unexpected tool")
	}
}

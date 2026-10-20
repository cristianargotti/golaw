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
)

func TestM2PinCommitsAndEnrollsEachVersion(t *testing.T) {
	t.Parallel()
	for _, version := range []string{"0.1.0", m2CandidateVersion, "0.3.0"} {
		s, repo := m2PinRepository(t)
		before, err := s.m2ObserveGit(t.Context(), repo, s.env, "rev-parse", "HEAD")
		if err != nil {
			t.Fatal(err)
		}
		if pinErr := s.m2CommitPin(t.Context(), repo, version); pinErr != nil {
			t.Fatal(pinErr)
		}
		after, err := s.m2ObserveGit(t.Context(), repo, s.env, "rev-parse", "HEAD")
		if err != nil || after == before {
			t.Fatalf("%s: pin did not make a commit: %v", version, err)
		}
		got := s.execute(t.Context(), command{
			program: s.git, args: []string{"show", "HEAD:" + policyFile}, dir: repo, env: s.env,
		})
		if got.code != 0 || got.err != nil {
			t.Fatalf("read committed pin: %+v", got)
		}
		m2RequirePinRecord(t, s, repo, version, got.stdout)
	}
}

func m2RequirePinRecord(t *testing.T, s *suite, repo, version, body string) {
	t.Helper()
	file, err := policy.Decode([]byte(body))
	if err != nil || file.Kit.Version != version {
		t.Fatalf("committed pin = %s, error = %v", file.Kit.Version, err)
	}
	root, _, err := install.ProjectIdentity(repo)
	if err != nil {
		t.Fatal(err)
	}
	record, found, err := policy.ReadRecord(m2Dirs(t, s, s.env).State, root)
	if err != nil || !found || record.KitVersion != version || record.PolicyDigest != policy.Digest(file) ||
		record.Bootstrap != body || !record.WrittenAt.Equal(m2Moment()) {
		t.Fatalf("pin enrollment = %+v, found=%v, error=%v", record, found, err)
	}
}

func TestM2PinPlumbingFailures(t *testing.T) {
	t.Parallel()
	for _, target := range []string{gitAdd, "write-tree", "HEAD", "commit-tree", "update-ref"} {
		s, repo := m2PinRepository(t)
		fixture := s.execute
		s.execute = func(ctx context.Context, c command) result {
			if c.program == s.git && slices.Contains(c.args, target) {
				return result{err: ErrAcceptance}
			}
			return fixture(ctx, c)
		}
		if err := s.m2CommitPin(t.Context(), repo, "0.1.0"); err == nil {
			t.Fatalf("%s: failed plumbing passed", target)
		}
	}
}

func m2PinRepository(t *testing.T) (*suite, string) {
	t.Helper()
	s := m2TestSuite(t)
	s.execute = m2FixtureExecutor(t, s)
	repo, err := s.repository(t.Context(), "pin", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.gitSteps(t.Context(), repo, []string{gitAdd, "."},
		commitArgs("test: prepare pinned repository")); err != nil {
		t.Fatal(err)
	}
	return s, repo
}

func TestM2PinRejectsMissingPolicyAndEnrollment(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"policy", "decode", "record", "broken-record", "dirs"} {
		s, repo := m2PinRepository(t)
		m2InvalidatePin(t, s, repo, mode)
		if err := s.m2CommitPin(t.Context(), repo, "0.1.0"); err == nil {
			t.Fatalf("%s: invalid policy state passed", mode)
		}
	}
}

func m2InvalidatePin(t *testing.T, s *suite, repo, mode string) {
	t.Helper()
	switch mode {
	case "policy":
		if err := os.Remove(filepath.Join(repo, policyFile)); err != nil {
			t.Fatal(err)
		}
	case "decode":
		m2Write(t, filepath.Join(repo, policyFile), "unknown: true\n")
	case "record", "broken-record":
		root, _, err := install.ProjectIdentity(repo)
		if err != nil {
			t.Fatal(err)
		}
		path := policy.RecordPath(m2Dirs(t, s, s.env).State, root)
		if mode == "record" {
			if removeErr := os.Remove(path); removeErr != nil {
				t.Fatal(removeErr)
			}
		} else {
			m2Write(t, path, "broken\n")
		}
	case "dirs":
		s.env = slices.DeleteFunc(s.env, func(entry string) bool {
			return strings.HasPrefix(entry, "HOME=") || strings.HasPrefix(entry, "XDG_") ||
				strings.HasPrefix(entry, "LOCALAPPDATA=") || strings.HasPrefix(entry, "APPDATA=")
		})
	}
}

func TestM2EqualRungsReceiveValidInputs(t *testing.T) {
	t.Parallel()
	for _, rungName := range []string{"pre-commit", "commit-msg", "pre-push"} {
		s, repo := m2PinRepository(t)
		name := rungName + "-equal"
		script := m2LifecycleScript(t, s, name, "answer")
		s.execute = func(ctx context.Context, c command) result {
			if c.program == s.binary && slices.Contains(c.args, commandHook) {
				m2RequireEqualInput(t, s, c, rungName)
			}
			return script.Execute(ctx, c)
		}
		if err := s.m2PinCase(t.Context(), repo, name); err != nil || script.Targets != 1 {
			t.Fatalf("%s: target=%d error=%v", name, script.Targets, err)
		}
	}
}

func m2RequireEqualInput(t *testing.T, s *suite, c command, rungName string) {
	t.Helper()
	if c.input != "" {
		t.Fatalf("equal rung input is not empty: %q", c.input)
	}
	if !strings.Contains(environmentValue(c.env, envPath), s.m2.Bin) {
		t.Fatal("equal rung lost verified tools")
	}
	if rungName == "commit-msg" {
		body, err := os.ReadFile(c.args[len(c.args)-1])
		if err != nil || string(body) != m2MessageBody {
			t.Fatalf("equal commit-msg input = %q, error = %v", body, err)
		}
	}
}

func TestM2PinRefusalsNeedBothVersionsAndRepair(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"pin", "candidate", "repair", "incomplete-stdout", "code", "process"} {
		s, repo := m2PinRepository(t)
		script := m2LifecycleScript(t, s, "pre-commit-older", "answer")
		s.execute = func(ctx context.Context, c command) result {
			got := script.Execute(ctx, c)
			if c.program == s.binary && slices.Contains(c.args, commandHook) {
				got = m2PinFault(got, mode)
			}
			return got
		}
		if err := s.m2PinCase(t.Context(), repo, "pre-commit-older"); err == nil || script.Targets != 1 {
			t.Fatalf("%s: target=%d error=%v", mode, script.Targets, err)
		}
	}
}

func m2PinFault(got result, mode string) result {
	switch mode {
	case "pin":
		got.stderr = strings.ReplaceAll(got.stderr, m2OlderVersion, "missing")
	case "candidate":
		got.stderr = strings.ReplaceAll(got.stderr, m2CandidateVersion, "missing")
	case "repair":
		got.stderr = strings.ReplaceAll(got.stderr, "init --update", "missing")
	case "incomplete-stdout":
		got.stdout = strings.ReplaceAll(got.stderr, m2CandidateVersion, "missing")
		got.stderr = ""
	case "code":
		got.code = 1
	case "process":
		got.err = ErrAcceptance
	}
	return got
}

func TestM2PinRejectsInvalidCasesAndEnrollmentBodies(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	for _, name := range []string{"missing", "unknown-older", "pre-commit-unknown"} {
		if err := s.m2PinCase(t.Context(), t.TempDir(), name); err == nil {
			t.Fatalf("%s: invalid pin case passed", name)
		}
	}
	if err := s.m2EnrollPolicy(t.TempDir(), []byte("unknown: true\n")); err == nil {
		t.Fatal("invalid enrollment policy passed")
	}
	if err := s.m2EnrollPolicy(filepath.Join(t.TempDir(), "missing"), []byte("golaw: 1\n")); err == nil {
		t.Fatal("missing enrollment repository passed")
	}
}

func TestM2PinRefusalsAcceptEitherDiagnosticChannel(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ row, channel string }{
		{"pre-commit-older", "stdout"},
		{"commit-msg-newer", "split"},
		{"pre-push-older", "stderr"},
	} {
		s, repo := m2PinRepository(t)
		script := m2LifecycleScript(t, s, tc.row, "answer")
		s.execute = func(ctx context.Context, c command) result {
			got := script.Execute(ctx, c)
			if c.program == s.binary && slices.Contains(c.args, commandHook) {
				got = m2PinDiagnosticChannel(t, got, tc.channel)
			}
			return got
		}
		if err := s.m2PinCase(t.Context(), repo, tc.row); err != nil || script.Targets != 1 {
			t.Fatalf("%s %s: target=%d error=%v", tc.row, tc.channel, script.Targets, err)
		}
	}
}

func m2PinDiagnosticChannel(t *testing.T, got result, channel string) result {
	t.Helper()
	switch channel {
	case "stdout":
		got.stdout, got.stderr = got.stderr, ""
	case "split":
		before, after, found := strings.Cut(got.stderr, ": install")
		if !found {
			t.Fatal("simulated refusal has no repair boundary")
		}
		got.stdout, got.stderr = before+":\n", "install"+after
	}
	return got
}

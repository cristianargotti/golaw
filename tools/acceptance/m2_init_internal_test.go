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

func m2LifecycleFixture(t *testing.T, name string, initialized bool) (*suite, string) {
	t.Helper()
	s := m2TestSuite(t)
	s.execute = m2LifecycleScript(t, s, name, "answer").Execute
	repo, err := s.m2LifecycleRepository(t.Context(), name)
	if err != nil {
		t.Fatal(err)
	}
	if initialized {
		if err := s.m2LifecycleBaseline(t.Context(), repo); err != nil {
			t.Fatal(err)
		}
	}
	return s, repo
}

func TestM2InitRejectsReceiptRoutingAndEnrollmentChanges(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"receipt", "routing", "enrollment", "output"} {
		s, repo := m2LifecycleFixture(t, "init-unarmed", false)
		s.execute = m2InitMutation(t, s, repo, mode)
		if mode == "output" {
			if err := s.m2InitIdempotent(t.Context(), repo); err == nil {
				t.Fatal("second init output passed")
			}
		} else if err := s.m2InitUnarmed(t.Context(), repo); err == nil {
			t.Fatalf("%s: init corruption passed", mode)
		}
	}
}

func TestM2InitAllowsUnchangedStatus(t *testing.T) {
	t.Parallel()
	s, repo := m2LifecycleFixture(t, "init-idempotent", false)
	fixture := s.execute
	s.execute = func(ctx context.Context, c command) result {
		got := fixture(ctx, c)
		if c.program == s.binary && slices.Contains(c.args, commandInit) {
			got.stdout = "golaw.yml: no changes\nenrolled: policy unchanged\n"
		}
		return got
	}
	if err := s.m2InitIdempotent(t.Context(), repo); err != nil {
		t.Fatalf("unchanged init status was refused: %v", err)
	}
}

func TestM2InitRejectsEnrollmentRecordMutation(t *testing.T) {
	t.Parallel()
	s, repo := m2LifecycleFixture(t, "init-idempotent", false)
	fixture, applies := s.execute, 0
	s.execute = func(ctx context.Context, c command) result {
		got := fixture(ctx, c)
		if c.program == s.binary && slices.Contains(c.args, commandInit) {
			applies++
			if applies == 2 {
				root, _, err := install.ProjectIdentity(repo)
				if err != nil {
					t.Fatal(err)
				}
				dirs := m2Dirs(t, s, c.env)
				m2Write(t, policy.RecordPath(dirs.State, root), "{}")
			}
		}
		return got
	}
	if err := s.m2InitIdempotent(t.Context(), repo); err == nil {
		t.Fatal("changed enrollment record passed")
	}
}

func TestM2LifecyclePreparationFailures(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	s.execute = func(context.Context, command) result { return result{err: ErrAcceptance} }
	if err := s.lifecycleM2(t.Context(), "unknown"); err == nil {
		t.Fatal("unknown case passed")
	}
	if err := s.lifecycleM2(t.Context(), "init-unarmed"); err == nil {
		t.Fatal("failed git initialization passed")
	}
	blocked := filepath.Join(t.TempDir(), "blocked")
	m2Write(t, blocked, "occupied")
	s.base = blocked
	if _, err := s.m2LifecycleRepository(t.Context(), "blocked"); err == nil {
		t.Fatal("blocked repository path passed")
	}
	if err := s.m2LifecycleBaseline(t.Context(), blocked); err == nil {
		t.Fatal("failed init passed")
	}
}

func TestM2EnrollmentRequiresPolicyAndMatchingRecord(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"missing-policy", "invalid-policy", "missing-record", "invalid-record", "version", "dirs"} {
		s, repo := m2LifecycleFixture(t, "init-unarmed", true)
		root, _, err := install.ProjectIdentity(repo)
		if err != nil {
			t.Fatal(err)
		}
		dirs := m2Dirs(t, s, s.env)
		switch mode {
		case "missing-policy":
			if err := os.Remove(filepath.Join(repo, policyFile)); err != nil {
				t.Fatal(err)
			}
		case "invalid-policy":
			m2Write(t, filepath.Join(repo, policyFile), "[")
		case "missing-record":
			if err := os.Remove(policy.RecordPath(dirs.State, root)); err != nil {
				t.Fatal(err)
			}
		case "invalid-record":
			m2Write(t, policy.RecordPath(dirs.State, root), "{}")
		case "version":
			m2Pin(t, repo, m2OlderVersion)
		case "dirs":
			s.env = nil
		}
		if err := s.m2CheckEnrollment(repo, s.env); err == nil {
			t.Fatalf("%s: incomplete enrollment passed", mode)
		}
	}
}

func TestM2InitObservationsPropagateFailures(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"before", "after", "init"} {
		s, repo := m2LifecycleFixture(t, "init-unarmed", false)
		s.execute = m2InitObservationFailure(s.execute, s.binary, s.git, mode)
		if err := s.m2InitUnarmed(t.Context(), repo); err == nil {
			t.Fatalf("%s: failed observation passed", mode)
		}
	}
}

func TestM2InitDigestFailures(t *testing.T) {
	t.Parallel()
	s, repo := m2LifecycleFixture(t, "init-idempotent", true)
	if got, err := m2OptionalDigests(filepath.Join(repo, "missing")); err != nil || len(got) != 0 {
		t.Fatalf("absent hooks: %v %v", got, err)
	}
	if _, err := s.m2InitDigests(filepath.Join(repo, "missing")); err == nil {
		t.Fatal("missing checkout passed")
	}
	s.env = nil
	if _, err := s.m2InitDigests(repo); err == nil {
		t.Fatal("missing state environment passed")
	}
	if err := s.m2SetupAbsent(repo, nil); err == nil {
		t.Fatal("missing receipt environment passed")
	}
	s.env = []string{"HOME=" + filepath.Join(repo, "missing")}
	if _, err := s.m2InitDigests(repo); err == nil {
		t.Fatal("missing state directory passed")
	}
}

func TestM2InitIdempotencePropagatesMissingSnapshots(t *testing.T) {
	t.Parallel()
	for _, at := range []int{1, 2} {
		s, repo := m2LifecycleFixture(t, "init-idempotent", false)
		fixture, applies := s.execute, 0
		s.execute = func(ctx context.Context, c command) result {
			got := fixture(ctx, c)
			if c.program == s.binary && strings.Contains(strings.Join(c.args, " "), commandInit) {
				applies++
				if applies == at {
					s.env = nil
				}
			}
			return got
		}
		if err := s.m2InitIdempotent(t.Context(), repo); err == nil {
			t.Fatalf("apply %d: missing snapshot passed", at)
		}
	}
}

func m2InitMutation(t *testing.T, s *suite, repo, mode string) executor {
	t.Helper()
	fixture := s.execute
	return func(ctx context.Context, c command) result {
		got := fixture(ctx, c)
		if c.program != s.binary || !slices.Contains(c.args, commandInit) {
			return got
		}
		switch mode {
		case "receipt":
			m2Setup(ctx, t, s, c, false)
		case "routing":
			routed := filepath.Join(s.base, "other-hooks")
			m2Write(t, filepath.Join(routed, "local"), m2LocalEdit)
			if err := s.gitSteps(ctx, repo, []string{"config", "core.hooksPath", routed}); err != nil {
				t.Fatal(err)
			}
		case "enrollment":
			m2Pin(t, repo, m2OlderVersion)
		case "output":
			got.stdout = "+++ golaw.yml\n"
		}
		return got
	}
}

func m2InitObservationFailure(fixture executor, binary, git, mode string) executor {
	applied := false
	return func(ctx context.Context, c command) result {
		if c.program == binary {
			applied = true
			if mode == "init" {
				return result{err: ErrAcceptance}
			}
		}
		if c.program == git && slices.Contains(c.args, "--git-path") &&
			(mode == "before" || mode == "after" && applied) {
			return result{err: ErrAcceptance}
		}
		return fixture(ctx, c)
	}
}

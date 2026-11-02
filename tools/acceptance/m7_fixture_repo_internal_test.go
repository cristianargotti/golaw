package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestM7RepositoryAdoptsTheFixtureAndRecordsItsBase: the contracts, and
// the recorded generator outputs only when asked, are committed with the
// adoption, and the base is the commit the repository is on.
func TestM7RepositoryAdoptsTheFixtureAndRecordsItsBase(t *testing.T) {
	t.Parallel()
	for _, thirdParty := range []bool{true, false} {
		s := newM7Suite(m2TestSuite(t))
		s.execute = m5Script(t, s.suite, m6InitReply()).Execute
		repo, err := s.m7Repository(t.Context(), "fresh", thirdParty)
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range []string{"go.mod", "internal/application/billing/ports.go", policyFile} {
			if _, statErr := os.Stat(filepath.Join(repo.dir, filepath.FromSlash(path))); statErr != nil {
				t.Fatalf("the adopted repository lacks %s: %v", path, statErr)
			}
		}
		_, statErr := os.Stat(filepath.Join(repo.dir, "api", "gen", "server.gen.go"))
		if (statErr == nil) != thirdParty {
			t.Fatalf("third party outputs written = %v, asked %v", statErr == nil, thirdParty)
		}
		head, err := s.m7Head(t.Context(), repo.dir)
		if err != nil || head != repo.base || len(head) < 40 {
			t.Fatalf("base %q, head %q, %v", repo.base, head, err)
		}
	}
}

// TestM7RepositoryStopsAtTheFirstFailingStep: a missing fixture, a write
// under a file, a refused git init, a refused adoption, a refused commit
// and an unreadable head each answer no repository.
func TestM7RepositoryStopsAtTheFirstFailingStep(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		mode func(t *testing.T, s *m7Suite) []m5Reply
		want string
	}{
		{"red a missing fixture", func(t *testing.T, s *m7Suite) []m5Reply {
			t.Helper()
			s.root = t.TempDir()
			return nil
		}, "read the M7 fixture repo"},
		{"red a base that is a file", func(t *testing.T, s *m7Suite) []m5Reply {
			t.Helper()
			s.base = filepath.Join(t.TempDir(), "base")
			m2Write(t, s.base, "x")
			return nil
		}, "create acceptance directory"},
		{"red a git init refused", func(_ *testing.T, s *m7Suite) []m5Reply {
			return []m5Reply{{program: s.git, prefix: []string{commandInit}, answer: m5Answer(result{code: 128})}}
		}, "exit 128"},
		{"red an adoption refused", func(*testing.T, *m7Suite) []m5Reply {
			return []m5Reply{{prefix: []string{commandInit}, answer: m5Answer(result{code: 2})}}
		}, "adopt the fixture repository refused"},
		{"red a commit refused", func(_ *testing.T, s *m7Suite) []m5Reply {
			return []m5Reply{m6InitReply(), {program: s.git, prefix: []string{gitAdd}, answer: m5Answer(result{code: 128})}}
		}, "exit 128"},
		{"red a head unreadable", func(_ *testing.T, s *m7Suite) []m5Reply {
			return []m5Reply{m6InitReply(), {program: s.git, prefix: []string{m2RevParse}, answer: m5Answer(result{code: 128})}}
		}, "read the head commit"},
	} {
		s := newM7Suite(m2TestSuite(t))
		s.execute = m5Script(t, s.suite, tc.mode(t, s)...).Execute
		if _, err := s.m7Repository(t.Context(), "refused", false); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: repository = %v, want an error naming %q", tc.name, err, tc.want)
		}
	}
}

// TestM7NewAllRunsEveryCommandAndNamesTheOneThatFailed: every command runs
// in order, and a refused one stops the rest and names itself.
func TestM7NewAllRunsEveryCommandAndNamesTheOneThatFailed(t *testing.T) {
	t.Parallel()
	s := newM7Suite(m2TestSuite(t))
	var ran []string
	s.execute = m5Script(t, s.suite, m5Reply{prefix: []string{"new"}, answer: func(_ *testing.T, c command) result {
		ran = append(ran, strings.Join(c.args, " "))
		return result{}
	}}).Execute
	if err := s.m7NewAll(t.Context(), t.TempDir()); err != nil || len(ran) != len(m7Commands()) {
		t.Fatalf("new all = %v after %q", err, ran)
	}
	s.execute = m5Script(t, s.suite,
		m5Reply{prefix: []string{"new", "usecase"}, once: true, answer: m5Answer(result{})},
		m5Reply{prefix: []string{"new", "port"}, answer: m5Answer(result{code: 2, stderr: "golaw: exists\n"})},
	).Execute
	err := s.m7NewAll(t.Context(), t.TempDir())
	if !errors.Is(err, ErrAcceptance) || !strings.Contains(err.Error(), "golaw new port growth.Clock --usecase ReadPlan") {
		t.Fatalf("a refused port = %v", err)
	}
}

// TestM7FixtureRefusesAFileWithoutTheSuffix: a fixture file a linter of
// the kit could read as the kit's own is refused by name.
func TestM7FixtureRefusesAFileWithoutTheSuffix(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	m2Write(t, filepath.Join(root, filepath.FromSlash(m7FixtureRoot), m7RepoDir, "plain.go"), "package plain\n")
	if _, err := m7Fixture(root, m7RepoDir); !errors.Is(err, ErrAcceptance) || !strings.Contains(err.Error(), "plain.go") {
		t.Fatalf("a bare fixture file = %v", err)
	}
}

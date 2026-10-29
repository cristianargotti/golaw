package main

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/surfacefix"
)

// TestM6RepositoryStopsAtTheFirstFailingStep: no adopted repository is
// answered when a write, the init of the candidate, the policy edit or the
// twin itself fails, and each refusal names its cause.
func TestM6RepositoryStopsAtTheFirstFailingStep(t *testing.T) {
	t.Parallel()
	s := m6TestSuite(t)
	blocked := map[string][]byte{"views": []byte("x\n"), "views/a.js": []byte("y\n")}
	if _, err := s.m6Repository(t.Context(), "blocked", blocked, nil); err == nil {
		t.Fatal("a file under a file was written")
	}
	s.execute = m5Script(t, s.suite, m5Reply{prefix: []string{commandInit}, answer: m5Answer(result{code: 2})}).Execute
	if _, err := s.m6Repository(t.Context(), "refused", map[string][]byte{"a.txt": []byte("a\n")}, nil); err == nil ||
		!strings.Contains(err.Error(), "adopt the fixture repository refused") {
		t.Fatalf("a refused init = %v", err)
	}
	s.execute = m5Script(t, s.suite, m5Reply{prefix: []string{commandInit}, answer: m5Answer(result{})}).Execute
	if _, err := s.m6Repository(t.Context(), "unwritten", map[string][]byte{"a.txt": []byte("a\n")}, m6WithoutFeedback); err == nil {
		t.Fatal("a policy init never wrote was edited")
	}
	s.execute = m5Script(t, s.suite, m6InitReply()).Execute
	if _, err := s.m6Repository(t.Context(), "bare", map[string][]byte{"a.txt": []byte("a\n")}, func([]byte) ([]byte, error) {
		return nil, ErrAcceptance
	}); !errors.Is(err, ErrAcceptance) {
		t.Fatalf("a failed edit = %v", err)
	}
	if _, err := s.m6Twin(t.Context(), "unknown", surfacefix.Twin(0), nil); !errors.Is(err, surfacefix.ErrTwin) {
		t.Fatalf("an unknown twin = %v", err)
	}
}

// TestM6RepositoryCommitFailureIsReported: an adoption git refuses to
// commit is no repository.
func TestM6RepositoryCommitFailureIsReported(t *testing.T) {
	t.Parallel()
	s := m6TestSuite(t)
	s.execute = m5Script(t, s.suite, m6InitReply(), m5Reply{program: s.git, prefix: []string{gitAdd}, answer: m5Answer(result{code: 128})}).Execute
	if _, err := s.m6Repository(t.Context(), "uncommitted", map[string][]byte{"a.txt": []byte("a\n")}, nil); err == nil {
		t.Fatal("an adoption git refused to stage was committed")
	}
}

// TestM6ClaudeWithoutTheBasePayloadRunsNothing: a module root without the
// base hook payload is an error before any command runs.
func TestM6ClaudeWithoutTheBasePayloadRunsNothing(t *testing.T) {
	t.Parallel()
	s := m6TestSuite(t)
	s.root = filepath.Join(t.TempDir(), "absent")
	s.execute = m5Script(t, s.suite).Execute
	if _, err := s.m6Claude(t.Context(), t.TempDir(), eventPre, "a.sh", []byte("x\n"), s.env); err == nil {
		t.Fatal("a hook call was built without its base payload")
	}
}

// TestM6ClaudeAnswersTheHookResult: the rows judge the hook's own answer,
// so m6Claude hands back what the candidate printed and exited with.
func TestM6ClaudeAnswersTheHookResult(t *testing.T) {
	t.Parallel()
	s := m6TestSuite(t)
	want := result{stdout: `{"decision":"block"}` + "\n", code: 2}
	s.execute = m5Script(t, s.suite, m5Reply{prefix: []string{commandHook, agentClaude}, answer: m5Answer(want)}).Execute
	got, err := s.m6Claude(t.Context(), t.TempDir(), eventPre, "views/a.js", []byte("x\n"), s.env)
	if err != nil || got != want {
		t.Fatalf("the hook answered %+v, %v; want %+v", got, err, want)
	}
}

package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestM2StagedRepositoryStates(t *testing.T) {
	t.Parallel()
	red := "first changed line\nsecond \u2014 line\n"
	prefix := strings.Repeat(m2Safe, 20)
	tests := []struct {
		name, path, head, index, checkout string
		missingHead, missingIndex         bool
	}{
		{"partial-red", pathNote, m2Safe, red, "first line\nsecond \u2014 line\n", false, false},
		{"partial-near", pathNote, m2Safe, m2Changed, red, false, false},
		{"opposite-red", pathNote, m2Safe, red, m2Changed, false, false},
		{"opposite-near", pathNote, m2Safe, m2Changed, red, false, false},
		{"deletion-red", "new.txt", "", red, m2Changed, true, false},
		{"deletion-near", pathNote, red, "", red, false, true},
		{"rename-red", "new.txt", "", prefix + red, prefix + m2Changed, true, false},
		{"rename-near", "new.txt", "", prefix + red, prefix + red, true, false},
		{"dirty-red", "dirty.txt", m2Safe, red, m2Safe, false, false},
		{"dirty-near", pathNote, m2Safe, m2Changed, m2Changed, false, false},
		{"alternate-red", pathNote, m2Safe, red, m2Changed, false, false},
		{"alternate-near", pathNote, m2Safe, m2Changed, red, false, false},
		{"length-red", "big.go", m2GoLines(300), m2GoLines(301), m2GoLines(301), false, false},
		{"length-near", "big.go", m2GoLines(320), m2GoLines(321), m2GoLines(321), false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s, repo := m2StateTestRepository(t, tc.name)
			m2RequireGitFile(t, s, repo, "HEAD:"+tc.path, tc.head, tc.missingHead)
			m2RequireGitFile(t, s, repo, ":"+tc.path, tc.index, tc.missingIndex)
			m2RequireCheckout(t, repo, tc.path, tc.checkout)
			m2RequireSpecialState(t, s, repo, tc.name, prefix, red)
		})
	}
}

func m2StateTestRepository(t *testing.T, name string) (*suite, string) {
	t.Helper()
	s := m2TestSuite(t).m2Scope(name)
	fixture := m2FixtureExecutor(t, s)
	s.execute = func(ctx context.Context, c command) result {
		if c.program == s.binary && slices.Equal(c.args, []string{commandInit, flagApply}) {
			m2RequireCheckout(t, c.dir, moduleFile, m2Module)
		}
		return fixture(ctx, c)
	}
	repo, err := s.m2StagedRepository(t.Context(), name)
	if err != nil {
		t.Fatal(err)
	}
	return s, repo
}

func m2RequireGitFile(t *testing.T, s *suite, repo, object, want string, missing bool) {
	t.Helper()
	got := s.execute(t.Context(), command{
		program: s.git, args: []string{"show", object}, dir: repo, env: s.env,
	})
	if got.err != nil || (got.code != 0) != missing || !missing && got.stdout != want {
		t.Fatalf("%s: result=%+v want=%q missing=%v", object, got, want, missing)
	}
}

func m2RequireCheckout(t *testing.T, repo, path, want string) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(repo, path))
	if err != nil || string(body) != want {
		t.Fatalf("checkout %s: %q, %v; want %q", path, body, err, want)
	}
}

func m2RequireSpecialState(t *testing.T, s *suite, repo, name, prefix, red string) {
	t.Helper()
	switch {
	case strings.HasPrefix(name, "rename"):
		want := prefix + m2Safe
		if name == "rename-near" {
			want = prefix + red
		}
		m2RequireGitFile(t, s, repo, "HEAD:old.txt", want, false)
		got := s.execute(t.Context(), command{
			program: s.git, args: []string{"diff", "--cached", "--name-status", "-M"},
			dir: repo, env: s.env,
		})
		if got.err != nil || got.code != 0 || !strings.HasPrefix(got.stdout, "R") ||
			!strings.Contains(got.stdout, "\told.txt\tnew.txt") {
			t.Fatalf("rename not classified: %+v", got)
		}
	case strings.HasPrefix(name, "deletion"):
		want := red
		if name == "deletion-red" {
			want = "removed record\nold \u2014 content\n"
		}
		m2RequireGitFile(t, s, repo, "HEAD:note.txt", want, false)
		m2RequireGitFile(t, s, repo, ":note.txt", "", true)
	case name == "dirty-near":
		m2RequireGitFile(t, s, repo, ":dirty.txt", m2Safe, false)
		m2RequireGitFile(t, s, repo, ":extra.txt", "", true)
		m2RequireCheckout(t, repo, "dirty.txt", red)
		m2RequireCheckout(t, repo, "extra.txt", red)
	case strings.HasPrefix(name, "alternate"):
		m2RequireAlternate(t, s, repo, name, red)
	}
}

func m2RequireAlternate(t *testing.T, s *suite, repo, name, red string) {
	t.Helper()
	for _, key := range []string{"GIT_DIR", "GIT_INDEX_FILE"} {
		value := environmentValue(s.env, key)
		if value == "" || filepath.IsAbs(value) {
			t.Fatalf("%s must be relative: %q", key, value)
		}
	}
	if environmentValue(s.env, "GIT_COMMON_DIR") == "" {
		t.Fatal("missing linked common directory")
	}
	if !slices.Contains(s.env, "GIT_PREFIX=") {
		t.Fatal("missing linked root prefix")
	}
	body, err := os.ReadFile(filepath.Join(repo, ".git"))
	if err != nil || !strings.HasPrefix(string(body), "gitdir: ") {
		t.Fatalf("not a linked worktree: %q %v", body, err)
	}
	original := s.env
	s.env = slices.DeleteFunc(slices.Clone(s.env), func(entry string) bool {
		return strings.HasPrefix(entry, "GIT_INDEX_FILE=")
	})
	want := m2Safe
	if name == "alternate-near" {
		want = red
	}
	m2RequireGitFile(t, s, repo, ":note.txt", want, false)
	s.env = original
}

func TestM2StagedRepositoryPolicy(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]string{"policy-red": "301", "policy-near": "299"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s, repo := m2StateTestRepository(t, name)
			body, err := os.ReadFile(filepath.Join(repo, policyFile))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Count(string(body), "file_lines: 300") != 1 {
				t.Fatal("original generated policy was not restored")
			}
			m2RequireGitFile(t, s, repo, "HEAD:"+policyFile, string(body), false)
			expected := strings.Replace(string(body), "file_lines: 300", "file_lines: "+want, 1)
			m2RequireGitFile(t, s, repo, ":"+policyFile, expected, false)
		})
	}
}

func TestM2StateCommandFailures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, row, program string
		args               []string
		at                 int
	}{
		{"git-init", "partial-red", "git", []string{commandInit}, 1},
		{"candidate-init", "partial-red", "candidate", []string{commandInit}, 1},
		{"baseline-add", "partial-red", "git", []string{gitAdd, "."}, 1},
		{"baseline-commit", "partial-red", "git", commitArgs("test: prepare staged fixture"), 1},
		{"state-add", "partial-red", "git", []string{gitAdd, "."}, 2},
		{"deletion", "deletion-red", "git", []string{"rm"}, 1},
		{"rename", "rename-red", "git", []string{"mv"}, 1},
		{"policy-add", "policy-red", "git", []string{gitAdd, "--", policyFile}, 1},
		{"linked-add", "alternate-red", "git", []string{"worktree", "add"}, 1},
		{"linked-init", "alternate-red", "candidate", []string{commandInit}, 2},
		{"default-index", "alternate-near", "git", []string{gitAdd, "--", pathNote}, 1},
		{"alternate-index", "alternate-red", "git", []string{"read-tree"}, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := m2TestSuite(t).m2Scope(tc.name)
			seen := m2StateFailCommand(t, s, tc.program, tc.args, tc.at)
			if _, err := s.m2StagedRepository(t.Context(), tc.row); err == nil || *seen != tc.at {
				t.Fatalf("error=%v command count=%d want=%d", err, *seen, tc.at)
			}
		})
	}
}

func m2StateFailCommand(t *testing.T, s *suite, kind string, args []string, at int) *int {
	t.Helper()
	fixture := m2FixtureExecutor(t, s)
	seen, program := 0, s.git
	if kind == "candidate" {
		program = s.binary
	}
	s.execute = func(ctx context.Context, c command) result {
		if c.program == program && len(c.args) >= len(args) && slices.Equal(c.args[:len(args)], args) {
			seen++
			if seen == at {
				return result{err: ErrAcceptance}
			}
		}
		return fixture(ctx, c)
	}
	return &seen
}

func TestM2StateFileFailures(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	s.execute = func(context.Context, command) result { return result{} }
	blocked := filepath.Join(t.TempDir(), "blocked")
	m2Write(t, blocked, "occupied")
	s.base = blocked
	if _, err := s.m2StagedRepository(t.Context(), "partial-red"); err == nil {
		t.Fatal("repository creation accepted a file parent")
	}
	if err := s.m2PopulateState(t.Context(), blocked, "partial-red", m2StagedFiles("partial-red")); err == nil {
		t.Fatal("state accepted an unwritable destination")
	}
	for _, body := range []string{"invalid: [", "kit: {}", "checks:\n  sizes:\n    file_lines: 299\n"} {
		repo := t.TempDir()
		m2Write(t, filepath.Join(repo, policyFile), body)
		if err := s.m2PolicyState(t.Context(), repo, "policy-red"); err == nil {
			t.Fatalf("policy accepted %q", body)
		}
	}
	if err := s.m2PolicyState(t.Context(), t.TempDir(), "policy-red"); err == nil {
		t.Fatal("missing policy accepted")
	}
	repo := t.TempDir()
	if err := s.m2LinkedEnvironment(repo, repo); err == nil {
		t.Fatal("missing linked pointer accepted")
	}
	m2Write(t, filepath.Join(repo, ".git"), "invalid pointer")
	if err := s.m2LinkedEnvironment(repo, repo); err == nil {
		t.Fatal("invalid linked pointer accepted")
	}
}

func TestM2DeletionIsAnAdditionAndDeletion(t *testing.T) {
	t.Parallel()
	s, repo := m2StateTestRepository(t, "deletion-red")
	got := s.execute(t.Context(), command{
		program: s.git, args: []string{"diff-index", "--cached", "--name-status", "-M", "HEAD", "--"},
		dir: repo, env: s.env,
	})
	if got.err != nil || got.code != 0 || got.stdout != "A\tnew.txt\nD\tnote.txt\n" {
		t.Fatalf("deletion fixture must add and delete, not rename: %+v", got)
	}
}

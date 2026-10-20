package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestM2SpawnRejectsPreparationFailures(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"linked", "tools", "fixture", "stage", "policy"} {
		s, repo, script := m2SpawnSuite(t, false)
		if mode == "tools" {
			s.m2.Ready = false
		}
		if mode == "fixture" {
			s.root = filepath.Join(s.base, "missing")
		}
		s.execute = m2SpawnPreparationFault(t, s, script, mode)
		if err := s.m2SpawnGit(t.Context(), repo, false); err == nil || script.Targets != 0 {
			t.Fatalf("%s: target=%d error=%v", mode, script.Targets, err)
		}
	}
}

func m2SpawnPreparationFault(t *testing.T, s *suite, script *m2Script, mode string) executor {
	t.Helper()
	return func(ctx context.Context, c command) result {
		if c.program == s.git && mode == "linked" && slices.Contains(c.args, "worktree") {
			return result{err: ErrAcceptance}
		}
		stage := c.program == s.git && slices.Contains(c.args, "add") && slices.Contains(c.args, m2SpawnFile)
		if stage && mode == "stage" {
			return result{err: ErrAcceptance}
		}
		got := script.Execute(ctx, c)
		if stage && mode == "policy" {
			m2Write(t, filepath.Join(c.dir, policyFile), "broken: [")
		}
		return got
	}
}

func TestM2SpawnRejectsChangedOrUnobservedState(t *testing.T) {
	t.Parallel()
	modes := []string{"before", "after", "head", "tree", "checkout", "default-index", "alternate-index", "unmeasured"}
	for _, mode := range modes {
		s, repo, script := m2SpawnSuite(t, false)
		s.execute = m2SpawnFaultExecutor(t, s, script, mode)
		if err := s.m2SpawnGit(t.Context(), repo, false); err == nil {
			t.Fatalf("%s: changed or unobserved state passed", mode)
		}
	}
}

func m2SpawnFaultExecutor(t *testing.T, s *suite, script *m2Script, mode string) executor {
	t.Helper()
	return func(ctx context.Context, c command) result {
		if slices.Contains(c.args, "--absolute-git-dir") &&
			(mode == "before" || mode == "after" && script.Targets > 0) {
			return result{err: ErrAcceptance}
		}
		got := script.Execute(ctx, c)
		if m2InstalledHook(s, c) {
			m2ChangeSpawnState(ctx, t, s, c, mode)
			if mode == "unmeasured" {
				return result{stdout: "test.staged unmeasured test process unavailable\n"}
			}
		}
		if script.Targets > 0 && c.program == s.git {
			return m2ChangeSpawnObservation(c, got, mode)
		}
		return got
	}
}

func m2ChangeSpawnObservation(c command, got result, mode string) result {
	if mode == "head" && slices.Contains(c.args, "--verify") {
		got.stdout = strings.Repeat("a", 40)
	}
	if mode == "tree" && slices.Contains(c.args, "write-tree") {
		got.stdout = strings.Repeat("b", 40)
	}
	return got
}

func m2ChangeSpawnState(ctx context.Context, t *testing.T, s *suite, c command, mode string) {
	t.Helper()
	var path string
	switch mode {
	case "checkout":
		path = filepath.Join(c.dir, m2SpawnFile)
	case "default-index":
		path = filepath.Join(c.dir, environmentValue(c.env, "GIT_DIR"), "index")
	case "alternate-index":
		path = filepath.Join(c.dir, environmentValue(c.env, "GIT_INDEX_FILE"))
	default:
		return
	}
	if mode != "checkout" {
		env := m2IndexTestEnvironment(c.env, path)
		if _, err := s.m2ObserveGit(ctx, c.dir, env, "ls-files", "--error-unmatch", "--", moduleFile); err != nil {
			t.Fatal(err)
		}
		got := s.execute(ctx, command{
			program: s.git, args: []string{"update-index", "--force-remove", "--", moduleFile},
			dir: c.dir, env: env,
		})
		if err := success(got); err != nil {
			t.Fatal(err)
		}
		return
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	m2Write(t, path, string(append(body, 0)))
}

func TestM2SpawnPolicyRejectsMissingOrAmbiguousSwitch(t *testing.T) {
	t.Parallel()
	s, repo := m2PinRepository(t)
	body, err := os.ReadFile(filepath.Join(repo, policyFile))
	if err != nil {
		t.Fatal(err)
	}
	for _, replacement := range []string{
		"", "broken: [", "schema: 1\n",
		strings.Replace(string(body), "test_staged: false", "test_staged: true", 1),
		string(body) + "\n# test_staged: false\n",
	} {
		path := filepath.Join(s.base, "policy-case", policyFile)
		m2Write(t, path, replacement)
		if replacement == "" {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		}
		if err := m2EnableStagedTests(filepath.Dir(path)); err == nil {
			t.Fatal("missing or ambiguous test switch passed")
		}
	}
}

func TestM2SpawnRequiresWritableFixture(t *testing.T) {
	t.Parallel()
	for _, afterStage := range []bool{false, true} {
		s, repo := m2PinRepository(t)
		fixture := s.execute
		block := func() {
			t.Helper()
			path := filepath.Join(repo, m2SpawnFile)
			if err := os.RemoveAll(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(path, dirMode); err != nil {
				t.Fatal(err)
			}
		}
		if !afterStage {
			block()
		}
		s.execute = func(ctx context.Context, c command) result {
			got := fixture(ctx, c)
			if afterStage && c.program == s.git && slices.Contains(c.args, "add") {
				block()
			}
			return got
		}
		if err := s.m2SpawnFiles(t.Context(), repo, false); err == nil {
			t.Fatalf("afterStage=%v: unwritable fixture passed", afterStage)
		}
	}
}

package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/runner"
)

func TestM2SpawnUsesLinkedStagedState(t *testing.T) {
	t.Parallel()
	for _, red := range []bool{false, true} {
		s, repo, script := m2SpawnSuite(t, red)
		s.execute = func(ctx context.Context, c command) result {
			if m2InstalledHook(s, c) {
				m2RequireSpawnEnvironment(t, c)
				m2RequireSpawnSources(ctx, t, s, c, red)
				m2RequireSpawnPolicy(ctx, t, s, c)
			}
			return script.Execute(ctx, c)
		}
		if err := s.m2SpawnGit(t.Context(), repo, red); err != nil || script.Targets != 1 {
			t.Fatalf("red=%v: target=%d error=%v", red, script.Targets, err)
		}
	}
}

func m2SpawnSuite(t *testing.T, red bool) (*suite, string, *m2Script) {
	t.Helper()
	s, repo := m2PinRepository(t)
	name := "spawn-git-near"
	if red {
		name = "spawn-git-red"
	}
	script := m2NativeScript(t, s, name, "answer")
	s.execute = script.Execute
	return s, repo, script
}

func m2RequireSpawnEnvironment(t *testing.T, c command) {
	t.Helper()
	for _, key := range runner.HookVariables() {
		count := 0
		for _, entry := range c.env {
			name, _, _ := strings.Cut(entry, "=")
			if strings.EqualFold(name, key) {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("hook variable %s appears %d times", key, count)
		}
	}
	if environmentValue(c.env, "GIT_WORK_TREE") != "." ||
		filepath.IsAbs(environmentValue(c.env, "GIT_DIR")) ||
		filepath.IsAbs(environmentValue(c.env, "GIT_INDEX_FILE")) {
		t.Fatal("spawn fixture lost relative linked paths")
	}
	info, err := os.Stat(filepath.Join(c.dir, gitDirectory))
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf("spawn checkout is not linked: %v", err)
	}
}

func m2RequireSpawnSources(ctx context.Context, t *testing.T, s *suite, c command, red bool) {
	t.Helper()
	passing, err := os.ReadFile(filepath.Join(s.root, dirTools, dirAcceptance, "testdata", "m2", "git_test.go.txt"))
	if err != nil {
		t.Fatal(err)
	}
	staged, err := s.m2ObserveGit(ctx, c.dir, c.env, "show", ":git_internal_test.go")
	if err != nil {
		t.Fatal(err)
	}
	checkout, err := os.ReadFile(filepath.Join(c.dir, "git_internal_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if red {
		if !strings.Contains(staged, `t.Fatal("staged failure reached")`) || !bytes.Equal(checkout, passing) {
			t.Fatal("red fixture did not stage failure over a passing checkout")
		}
	} else if staged != strings.TrimSpace(string(passing)) ||
		!bytes.Contains(checkout, []byte(`t.Fatal("staged failure reached")`)) {
		t.Fatal("near fixture did not stage child Git over a failing checkout")
	}
	env := slices.DeleteFunc(slices.Clone(c.env), func(entry string) bool {
		key, _, _ := strings.Cut(entry, "=")
		return key == "GIT_INDEX_FILE"
	})
	got := s.execute(ctx, command{
		program: s.git, args: []string{"ls-files", "--error-unmatch", "--", "git_internal_test.go"}, dir: c.dir, env: env,
	})
	if got.err != nil || got.code != 1 {
		t.Fatalf("default index contains the staged probe: %+v", got)
	}
}

func m2RequireSpawnPolicy(ctx context.Context, t *testing.T, s *suite, c command) {
	t.Helper()
	for _, ref := range []string{"HEAD:" + policyFile, ":" + policyFile} {
		body, err := s.m2ObserveGit(ctx, c.dir, c.env, "show", ref)
		if err != nil {
			t.Fatal(err)
		}
		file, err := policy.Decode([]byte(body))
		if err != nil || file.Ladder == nil || file.Ladder.TestStaged == nil || *file.Ladder.TestStaged {
			t.Fatalf("%s: committed or staged test switch changed: %v", ref, err)
		}
	}
	body, err := os.ReadFile(filepath.Join(c.dir, policyFile))
	if err != nil {
		t.Fatal(err)
	}
	file, err := policy.Decode(body)
	if err != nil || file.Ladder == nil || file.Ladder.TestStaged == nil || !*file.Ladder.TestStaged {
		t.Fatalf("checkout test switch was not enabled: %v", err)
	}
}

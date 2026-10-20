package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/runner"
)

func TestM2SpawnPreservesCompilerSearchPath(t *testing.T) {
	t.Parallel()
	for _, system := range []string{"linux", goosWindows} {
		t.Run(system, func(t *testing.T) {
			t.Parallel()
			s, repo, script := m2SpawnSuite(t, false)
			s.m2OS = system
			original := m2SpawnCompilerPath(t, s)
			var setupPath string
			s.execute = func(ctx context.Context, c command) result {
				if c.program == s.binary && slices.Equal(c.args, []string{m2SetupCommand}) {
					setupPath = environmentValue(c.env, envPath)
				}
				if m2InstalledHook(s, c) {
					m2RequireSpawnSearchPath(t, s, c.env, setupPath, original)
				}
				return script.Execute(ctx, c)
			}
			if err := s.m2SpawnGit(t.Context(), repo, false); err != nil || script.Targets != 1 {
				t.Fatalf("spawn target=%d error=%v", script.Targets, err)
			}
		})
	}
}

func m2SpawnCompilerPath(t *testing.T, s *suite) string {
	t.Helper()
	dirs := []string{filepath.Join(s.base, "compiler bin"), filepath.Join(s.base, "compiler support")}
	for _, name := range []string{"cc", "go", m2GitCommand, "sh", binaryName, "golangci-lint", "lefthook", "gitleaks"} {
		m2Write(t, filepath.Join(dirs[0], executable(name)), "simulated fallback executable\n")
	}
	original := strings.Join(dirs, string(os.PathListSeparator))
	s.env = slices.DeleteFunc(s.env, func(entry string) bool {
		key, _, _ := strings.Cut(entry, "=")
		return strings.EqualFold(key, envPath)
	})
	s.env = append(s.env, "Path="+original)
	return original
}

func m2RequireSpawnSearchPath(t *testing.T, s *suite, env []string, setupPath, original string) {
	t.Helper()
	want := setupPath + string(os.PathListSeparator) + original
	if setupPath == "" || environmentValue(env, envPath) != want {
		t.Fatalf("spawn PATH did not append original PATH after verified setup paths: got=%q want=%q",
			environmentValue(env, envPath), want)
	}
	for _, tool := range runner.Toolchain() {
		got, err := resolveExecutable(tool.Name, env)
		if err != nil || got != filepath.Join(s.m2.Bin, executable(tool.Name)) {
			t.Fatalf("verified tool %s lost PATH precedence: %s %v", tool.Name, got, err)
		}
	}
	for _, name := range []string{"go", m2GitCommand, "sh", binaryName} {
		got, err := resolveExecutable(name, env)
		if err != nil || filepath.Dir(got) == filepath.SplitList(original)[0] {
			t.Fatalf("prerequisite %s lost PATH precedence: %s %v", name, got, err)
		}
	}
	got, err := resolveExecutable("cc", env)
	if err != nil || got != filepath.Join(filepath.SplitList(original)[0], executable("cc")) {
		t.Fatalf("compiler fallback was not resolvable: %s %v", got, err)
	}
}

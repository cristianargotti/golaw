package main

import (
	"bytes"
	"context"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestM2PushEnvironmentKeepsOnlyRequiredTools(t *testing.T) {
	t.Parallel()
	for _, system := range []string{"linux", goosWindows} {
		for _, red := range []bool{false, true} {
			s, _ := m2PrerequisiteSuite(t, system)
			repo := t.TempDir()
			original := slices.Clone(s.env)
			shared := m2DatabaseDigests(t, s.m2.Bin)
			builds := 0
			s.execute = func(_ context.Context, c command) result {
				builds++
				m2RequireScannerBuild(t, s, repo, c)
				return simulatedBuild(t, c)
			}
			env, err := s.m2PushEnvironment(t.Context(), repo, s.env, red)
			if err != nil {
				t.Fatal(err)
			}
			if builds != 1 || !slices.Equal(s.env, original) || !maps.Equal(shared, m2DatabaseDigests(t, s.m2.Bin)) {
				t.Fatal("proxy preparation changed shared tools or the original environment")
			}
			m2RequireDatabaseEnvironment(t, s, env, original, red)
			m2RequirePushTools(t, s, env)
		}
	}
}

func m2RequireScannerBuild(t *testing.T, s *suite, repo string, c command) {
	t.Helper()
	if c.program != s.goTool || c.dir != repo || len(c.args) != 5 ||
		!slices.Equal(c.args[:3], []string{goBuild, m2Trimpath, "-o"}) {
		t.Fatalf("unexpected scanner build: %+v", c)
	}
	if c.args[3] != filepath.Join(s.base, m2ScannerProxyDir, executable(m2Scanner)) {
		t.Fatalf("scanner output escaped case directory: %q", c.args)
	}
	source, err := os.ReadFile(c.args[4])
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile(filepath.Join(s.root, "tools", "acceptance", "testdata", "m2", "govulncheck_proxy.go.txt"))
	if err != nil || !bytes.Equal(source, fixture) {
		t.Fatalf("proxy source differs from the supplied passthrough: %v", err)
	}
}

func m2RequireDatabaseEnvironment(t *testing.T, s *suite, env, original []string, red bool) {
	t.Helper()
	for _, key := range []string{"HOME", "GOCACHE", "GOMODCACHE", "SystemRoot", "COMSPEC"} {
		if environmentValue(env, key) != environmentValue(original, key) {
			t.Fatalf("database preparation changed %s", key)
		}
	}
	scanner := filepath.Join(s.m2.Bin, executable(m2Scanner))
	if got := environmentValue(env, m2RealScannerKey); got != scanner || !filepath.IsAbs(got) {
		t.Fatalf("real scanner = %q, want absolute %q", got, scanner)
	}
	database, err := url.Parse(environmentValue(env, m2DatabaseKey))
	if err != nil {
		t.Fatal(err)
	}
	want := "empty-db"
	if red {
		want = "red-db"
	}
	if database.Scheme != "file" || database.Host != "" || !strings.HasSuffix(database.Path, "/"+want) {
		t.Fatalf("database = %s, want local %s", database, want)
	}
	if environmentValue(env, "GOVULNDB") != "" {
		t.Fatal("database selection used GOVULNDB")
	}
}

func m2RequirePushTools(t *testing.T, s *suite, env []string) {
	t.Helper()
	for _, name := range []string{m2Scanner, "lefthook", "go", m2GitCommand, "sh", binaryName} {
		if _, err := resolveExecutable(name, env); err != nil {
			t.Fatalf("push lost %s: %v", name, err)
		}
	}
	for _, name := range []string{"gitleaks", "golangci-lint"} {
		if path, err := resolveExecutable(name, env); err == nil {
			t.Fatalf("push can invoke unrelated tool %s at %s", name, path)
		}
	}
	paths := filepath.SplitList(environmentValue(env, envPath))
	if paths[0] != filepath.Join(s.base, m2ScannerProxyDir) || slices.Contains(paths, s.m2.Bin) {
		t.Fatalf("push retained shared tool routing: %q", paths)
	}
	original, err := os.ReadFile(filepath.Join(s.m2.Bin, executable("lefthook")))
	if err != nil {
		t.Fatal(err)
	}
	copied, err := os.ReadFile(filepath.Join(s.base, m2PushToolsDir, executable("lefthook")))
	if err != nil || !bytes.Equal(original, copied) {
		t.Fatalf("push did not retain verified lefthook bytes: %v", err)
	}
}

func m2DatabaseDigests(t *testing.T, dir string) map[string]string {
	t.Helper()
	got, err := treeDigests(dir)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestM2DatabaseEnvironmentPreservesSetupAndTrustedPaths(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	repo := t.TempDir()
	m2Write(t, filepath.Join(repo, "receipt.json"), "owned setup receipt\n")
	m2Write(t, filepath.Join(repo, "hooks", "pre-push"), "owned push hook\n")
	before := m2DatabaseDigests(t, repo)
	env := append(prependPath(s.env, s.m2.Bin),
		"govulndb=https://unwanted.invalid", "golaw_m2_database=old", "golaw_m2_real_govulncheck=old")
	original := slices.Clone(env)
	s.execute = m2FixtureExecutor(t, s)
	got, err := s.m2DatabaseEnvironment(t.Context(), repo, env, false)
	if err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(before, m2DatabaseDigests(t, repo)) || !slices.Equal(env, original) {
		t.Fatal("database proxy changed setup or its caller's environment")
	}
	paths := filepath.SplitList(environmentValue(got, envPath))
	if !slices.Equal(paths[1:], filepath.SplitList(environmentValue(env, envPath))) {
		t.Fatalf("database proxy displaced trusted PATH directories: %q", paths)
	}
	m2RequireDatabaseEnvironment(t, s, got, env, false)
	for _, key := range []string{m2RealScannerKey, m2DatabaseKey} {
		count := 0
		for _, entry := range got {
			name, _, _ := strings.Cut(entry, "=")
			if strings.EqualFold(name, key) {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("%s occurs %d times", key, count)
		}
	}
}

func TestM2PushRejectsWindowsSourceDirectoryPollution(t *testing.T) {
	t.Parallel()
	for _, system := range []string{"linux", goosWindows} {
		for _, tool := range []string{"gitleaks", "golangci-lint"} {
			s, _ := m2PrerequisiteSuite(t, system)
			m2Write(t, filepath.Join(filepath.Dir(s.git), executable(tool)), "unrelated program\n")
			s.execute = m2FixtureExecutor(t, s)
			_, err := s.m2PushEnvironment(t.Context(), t.TempDir(), s.env, false)
			if (err != nil) != (system == goosWindows) {
				t.Fatalf("%s %s: pollution error = %v", system, tool, err)
			}
		}
	}
}

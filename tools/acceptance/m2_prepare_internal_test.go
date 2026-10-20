package main

import (
	"bytes"
	"context"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/install"
)

func TestM2CandidateCarriesAReleaseVersion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		mode string
		fail bool
	}{
		{"red version", "wrong-version", true},
		{"near release", "valid", false},
		{"corrupted answer", "corrupt", true},
		{"broken execution", "broken", true},
	}
	for _, tt := range tests {
		s := m2TestSuite(t)
		script := m2PrepareScript(t, s, tt.mode)
		s.execute = script.Execute
		err := s.prepareM2(t.Context())
		if (err != nil) != tt.fail {
			t.Fatalf("%s: prepare error = %v", tt.name, err)
		}
		if script.Calls == 0 {
			t.Fatalf("%s: preparation did not use its executor", tt.name)
		}
		if !tt.fail && (script.Targets == 0 || len(s.digest) != 64) {
			t.Fatalf("%s: candidate evidence missing", tt.name)
		}
	}
}

func TestM2ScopesShareCachesButNotHomes(t *testing.T) {
	t.Parallel()
	for _, parent := range [][]string{
		nil,
		{"GOCACHE=parent-build", "GOMODCACHE=parent-module"},
	} {
		base := t.TempDir()
		first := m2Environment(parent, filepath.Join(base, "one"),
			filepath.Join(base, "bin"), base)
		second := m2Environment(parent, filepath.Join(base, "two"),
			filepath.Join(base, "bin"), base)
		for _, key := range []string{"GOCACHE", "GOMODCACHE"} {
			left := environmentValue(first, key)
			right := environmentValue(second, key)
			if left == "" || left != right {
				t.Fatalf("%s is not shared: %q %q", key, left, right)
			}
			if !strings.HasPrefix(left, base+string(filepath.Separator)) {
				t.Fatalf("%s escaped acceptance base", key)
			}
		}
		if environmentValue(first, "HOME") == environmentValue(second, "HOME") {
			t.Fatal("case homes are shared")
		}
	}
}

func TestM2ReportKeepsEvidenceIdentity(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name     string
		evidence map[string]rowEvidence
		want     string
	}{
		{"existing identity", nil, "\tclaude\t"},
		{"git identity", map[string]rowEvidence{
			"m2-rung/pre-commit": {
				Agent: "git", Version: "git version 2.44.0",
				Binary: strings.Repeat("b", 64),
			},
		}, "\tgit\t"},
	} {
		var out bytes.Buffer
		rows := []row{{
			category: "m2-rung", name: "pre-commit",
			status: statusPass, detail: "installed invocation",
			required: true,
		}}
		if !reportEvidence(&out, rows, "2.1.299",
			strings.Repeat("a", 64), tt.evidence) {
			t.Fatalf("%s: report failed", tt.name)
		}
		if !strings.Contains(out.String(), tt.want) {
			t.Fatalf("%s: identity missing", tt.name)
		}
	}
}

func TestM2CandidateInstallationIsIsolated(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	script := m2PrepareScript(t, s, "valid")
	s.execute = func(ctx context.Context, c command) result {
		if c.program == s.goTool {
			m2CheckCandidateInstall(t, s, c)
		}
		got := script.Execute(ctx, c)
		if c.program == s.binary {
			got.stdout = "golaw 0.2.0\r\nmodule: example.invalid/module\r\n" +
				"build: source build, executable not verifiable\r\n"
		}
		return got
	}
	if err := s.prepareM2(t.Context()); err != nil {
		t.Fatal(err)
	}
	if script.Targets != 1 || s.digest != install.Digest([]byte(m2Executable)) ||
		s.m2.Live || s.m2.Ready {
		t.Fatalf("candidate state = %+v, probes = %d", s.m2, script.Targets)
	}
}

func TestM2CandidateRejectsInvalidVersionBeforeEffects(t *testing.T) {
	t.Parallel()
	for _, version := range []string{"", "latest", "v0.2.0", "0.2", "../0.2.0"} {
		s := m2TestSuite(t)
		script := m2PrepareScript(t, s, "valid")
		s.execute = script.Execute
		before, err := treeDigests(s.base)
		if err != nil {
			t.Fatal(err)
		}
		if _, versionErr := s.versionedBinary(t.Context(), version); !errors.Is(versionErr, ErrAcceptance) {
			t.Fatalf("%q: invalid version error = %v", version, versionErr)
		}
		after, err := treeDigests(s.base)
		if err != nil || script.Calls != 0 || !maps.Equal(before, after) {
			t.Fatalf("%q: invalid version had effects: %v", version, err)
		}
	}
}

func TestM2CandidateFailures(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"lookup", "enumerate", "proxy", "install", "readback", "limitation", "cancel"} {
		s := m2TestSuite(t)
		s.execute = m2PreparationFailure(t, s, mode)
		if mode == "lookup" {
			s.lookup = func(string) (string, error) { return "", ErrAcceptance }
		}
		ctx, cancel := context.WithCancel(t.Context())
		if mode == "cancel" {
			cancel()
		}
		err := s.prepareM2(ctx)
		cancel()
		if err == nil {
			t.Fatalf("%s: candidate failure passed", mode)
		}
	}
}

func TestM2ScopesKeepPrivateState(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	s.execute = func(context.Context, command) result { return result{stdout: "inherited"} }
	for _, name := range []string{"first", "second"} {
		scoped := s.m2Scope(name)
		if scoped.base != filepath.Join(s.m2.Base, "cases", name) ||
			scoped.home != filepath.Join(scoped.base, "home") || scoped.m2 != s.m2 ||
			scoped.m2OS != s.m2OS || scoped.binary != s.binary || scoped.digest != s.digest ||
			scoped.git != s.git || scoped.goTool != s.goTool || scoped.lookup == nil {
			t.Fatalf("scope = %+v", scoped)
		}
		for _, key := range []string{
			"HOME", "USERPROFILE", "APPDATA", "LOCALAPPDATA",
			"XDG_CONFIG_HOME", "XDG_STATE_HOME", "CODEX_HOME", "CLAUDE_CONFIG_DIR",
		} {
			if !strings.HasPrefix(environmentValue(scoped.env, key), scoped.home) {
				t.Fatalf("%s is not private", key)
			}
		}
		if scoped.execute(t.Context(), command{}).stdout != "inherited" ||
			environmentValue(scoped.env, "GOCACHE") != environmentValue(s.env, "GOCACHE") ||
			scoped.hookFile != "" || len(scoped.hooks) != 0 {
			t.Fatal("scope did not preserve only shared state")
		}
	}
}

func TestM2ExecutionPreservesBounds(t *testing.T) {
	t.Parallel()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	c := command{
		program: binary, args: []string{"-test.run=^TestAcceptanceChild$"},
		env: []string{"GOLAW_ACCEPTANCE_CHILD=1", "GOCOVERDIR=" + t.TempDir()}, input: "M2",
	}
	got := runM2Command(t.Context(), c)
	if got.err != nil || got.code != 2 || got.stdout != "M2" || got.stderr != "denied" {
		t.Fatalf("M2 execution = %+v", got)
	}
	if got := runCommandWithin(t.Context(), c, -time.Second); !errors.Is(got.err, context.DeadlineExceeded) {
		t.Fatalf("deadline = %+v", got)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if got := runM2Command(ctx, c); !errors.Is(got.err, context.Canceled) {
		t.Fatalf("cancellation = %+v", got)
	}
}

func TestM2EnvironmentReplacesMixedCaseCaches(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	env := m2Environment([]string{
		"gocache=first", "GOCACHE=second", "GoModCache=first", "GOMODCACHE=second",
	}, filepath.Join(base, "home"), filepath.Join(base, "bin"), base)
	for key, leaf := range map[string]string{"GOCACHE": "go-build", "GOMODCACHE": "go-mod"} {
		count := 0
		for _, entry := range env {
			name, value, _ := strings.Cut(entry, "=")
			if strings.EqualFold(name, key) {
				count++
				if value != filepath.Join(base, "caches", leaf) {
					t.Fatalf("%s retained an inherited cache", key)
				}
			}
		}
		if count != 1 {
			t.Fatalf("%s appears %d times", key, count)
		}
	}
}

func TestM2ReportFailsWhenRowCannotBeWritten(t *testing.T) {
	t.Parallel()
	rows := []row{{category: "m2", name: "row", status: statusPass}}
	out := &m2RowFailureWriter{}
	if reportEvidence(out, rows, "", "", nil) {
		t.Fatal("failed row writer passed")
	}
}

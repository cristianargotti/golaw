package main

import (
	"bytes"
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// m3PrepareAnswer answers the preparation of the M3 candidate: the tracked
// names, the install of the candidate and its version, and the tools
// repository's init, setup and probes through the M2 lifecycle answer.
func m3PrepareAnswer(t *testing.T, s *suite, version string) m2Answer {
	t.Helper()
	lifecycle := m2LifecycleAnswer(t, s, m2SetupCleanCase, "answer")
	return func(ctx context.Context, c command) (m2Reply, bool) {
		switch {
		case c.program == s.git && slices.Contains(c.args, "ls-files"):
			return m2Reply{got: result{stdout: "go.mod\x00cmd/golaw/main.go\x00"}}, true
		case c.program == s.goTool && len(c.args) > 0 && c.args[0] == "install":
			m2Write(t, filepath.Join(environmentValue(c.env, "GOBIN"), executable(binaryName)), m2Executable)
			return m2Reply{}, true
		case c.program == s.binary && slices.Equal(c.args, []string{"version"}):
			return m2Reply{got: result{stdout: "golaw " + version + "\nbuild: source build, executable not verifiable\n"}, target: true}, true
		}
		return lifecycle(ctx, c)
	}
}

func TestM3PreparationInstallsTheToolsAndTheOverlay(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, version string
		fail          bool
	}{
		{"near the M3 candidate", m3CandidateVersion, false},
		{"red another version", "0.2.0", true},
	} {
		s := m2TestSuite(t)
		s.m2.Ready = false
		s.execute = newM2Script(t, s, m3PrepareAnswer(t, s, tc.version)).Execute
		err := s.prepareM3(t.Context())
		if (err != nil) != tc.fail {
			t.Fatalf("%s: prepare = %v", tc.name, err)
		}
		if tc.fail {
			continue
		}
		proxy := environmentValue(s.env, "GOPROXY")
		if !s.m2.Ready || !strings.HasPrefix(proxy, "file:///") || !strings.HasSuffix(proxy, ",https://proxy.golang.org,direct") ||
			environmentValue(s.env, m2DatabaseKey) == "" || environmentValue(s.env, "GOBIN") != s.m2.Bin ||
			filepath.Base(s.m2.Make) != executable("make") || len(s.digest) != 64 {
			t.Fatalf("%s: environment %q, make %q, digest %q", tc.name, s.env, s.m2.Make, s.digest)
		}
	}
}

func TestM3OverlaySurvivesTheHomeOfACase(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	env := append(slices.Clone(s.env), m2RealScannerKey+"=/tools/govulncheck", m2DatabaseKey+"=file:///db")
	s.m2.Overlay = m3Overlay(env, s.m2.Bin, "file:///proxy")
	scope := s.m3Scope("m3-policy-baseline")
	for key, want := range map[string]string{
		"GOBIN": s.m2.Bin, "GOPROXY": "file:///proxy,https://proxy.golang.org,direct",
		"GONOSUMDB": "github.com/cristianargotti/golaw", "GOSUMDB": "sum.golang.org", "GOFLAGS": "-modcacherw",
		m2RealScannerKey: "/tools/govulncheck", m2DatabaseKey: "file:///db",
		"HOME": filepath.Join(s.m2.Base, "cases", "m3-policy-baseline", "home"),
	} {
		if got := environmentValue(scope.env, key); got != want {
			t.Fatalf("%s = %q, want %q", key, got, want)
		}
	}
	if got := withOverlay([]string{"gobin=/stale", "PATH=/bin"}, []string{"GOBIN=/tools"}); !slices.Equal(got, []string{"PATH=/bin", "GOBIN=/tools"}) {
		t.Fatalf("overlay = %q", got)
	}
}

func TestRunArgsRoutesTheM3Flag(t *testing.T) {
	t.Parallel()
	failing := func(context.Context, command) result { return result{err: ErrAcceptance} }
	lookup := func(name string) (string, error) { return filepath.Join(t.TempDir(), name), nil }
	var out bytes.Buffer
	if code := runArgs(&out, failing, lookup, []string{"-m3"}); code != 1 ||
		!strings.Contains(out.String(), "m3-preparation\t") || !strings.Contains(out.String(), "\tFAIL\trequired\tcandidate\t") {
		t.Fatalf("a failed M3 candidate = %d\n%s", code, out.String())
	}
	called := false
	refusing := func(context.Context, command) result {
		called = true
		return result{err: ErrAcceptance}
	}
	if code := runArgs(&out, refusing, lookup, []string{"-m3", "unexpected"}); code != 1 || called {
		t.Fatalf("an extra argument prepared the candidate: %d %v", code, called)
	}
}

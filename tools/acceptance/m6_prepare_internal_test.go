package main

import (
	"fmt"
	"path/filepath"
	"slices"
	"testing"
)

// TestM6ToolsInstallSeesTheCallersRuntimes: the setup of the surface tools
// runs on the isolated prerequisite PATH plus the directories of the node and
// python3 the caller resolves, since the kit never installs either runtime.
func TestM6ToolsInstallSeesTheCallersRuntimes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, missing string }{
		{"near both runtimes resolved", ""},
		{"near python3 absent", "python3"},
	} {
		s := newM6Suite(m2TestSuite(t))
		runtimes := map[string]string{"node": filepath.Join(s.base, "node-bin"), "python3": filepath.Join(s.base, "python-bin")}
		parent := s.lookup
		s.lookup = func(name string) (string, error) {
			dir, found := runtimes[name]
			switch {
			case found && name == tc.missing:
				return "", fmt.Errorf("%s: %w", name, ErrAcceptance)
			case found:
				return filepath.Join(dir, executable(name)), nil
			}
			return parent(name)
		}
		var path string
		s.execute = m5Script(t, s.suite,
			m5Reply{prefix: []string{commandInit}, once: true, answer: m5Answer(result{})},
			m5Reply{prefix: []string{m6SetupCommand, m6SurfaceAll}, once: true, answer: func(_ *testing.T, c command) result {
				path = environmentValue(c.env, envPath)
				return result{}
			}},
		).Execute
		if err := s.m6Tools(t.Context()); err != nil {
			t.Fatalf("%s: install = %v", tc.name, err)
		}
		dirs := filepath.SplitList(path)
		for name, dir := range runtimes {
			if slices.Contains(dirs, dir) == (name == tc.missing) {
				t.Errorf("%s: PATH %q and the %s directory %q", tc.name, path, name, dir)
			}
		}
		if len(dirs) == 0 || dirs[0] != s.m2.Bin {
			t.Errorf("%s: PATH %q no longer starts with the isolated directories", tc.name, path)
		}
	}
}

// TestM6PreparationInstallsTheToolsAndTheOverlay drives the preparation of
// the M6 candidate through the simulated answers of the M3 one: the Go
// tools installed once and the candidate's proxy ahead of the public one.
func TestM6PreparationInstallsTheToolsAndTheOverlay(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, version string
		fail          bool
	}{
		{"near the M6 candidate", m6CandidateVersion, false},
		{"red another version", m3CandidateVersion, true},
	} {
		s := m2TestSuite(t)
		s.m2.Ready = false
		s.execute = newM2Script(t, s, m3PrepareAnswer(t, s, tc.version)).Execute
		err := s.prepareM6(t.Context())
		if (err != nil) != tc.fail {
			t.Fatalf("%s: prepare = %v", tc.name, err)
		}
		if tc.fail {
			continue
		}
		proxy, err := m2FileURL(filepath.Join(s.base, m5ProxyDir))
		if err != nil {
			t.Fatal(err)
		}
		if !s.m2.Ready || environmentValue(s.env, "GOPROXY") != proxy+",https://proxy.golang.org,direct" ||
			environmentValue(s.env, m2DatabaseKey) == "" || len(s.digest) != 64 {
			t.Fatalf("%s: environment %q, digest %q", tc.name, s.env, s.digest)
		}
		scope := newM6Suite(s).m6Scope("m6-sample", "case")
		if got := environmentValue(scope.env, "GOPROXY"); got != environmentValue(s.env, "GOPROXY") || scope.home == s.home {
			t.Fatalf("%s: a case lost the overlay or shares the home: GOPROXY = %q", tc.name, got)
		}
	}
}

// TestM6ToolsInstallOnceIntoTheSharedDataHome: the surface tools install
// once per run, into the data home every tool row shares, and a failed
// install is remembered with its cause instead of retried by every row.
func TestM6ToolsInstallOnceIntoTheSharedDataHome(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		code int
		fail bool
	}{
		{"near the tools installed", 0, false},
		{"red the download refused", 2, true},
	} {
		s := newM6Suite(m2TestSuite(t))
		var data []string
		s.execute = m5Script(t, s.suite,
			m5Reply{prefix: []string{commandInit}, once: true, answer: m5Answer(result{})},
			m5Reply{prefix: []string{m6SetupCommand, m6SurfaceAll}, once: true, answer: func(_ *testing.T, c command) result {
				data = append(data, environmentValue(c.env, envDataHome))
				return result{code: tc.code, stderr: fmt.Sprintf("golaw: exit %d\n", tc.code)}
			}},
		).Execute
		first, second := s.m6Tools(t.Context()), s.m6Tools(t.Context())
		if (first != nil) != tc.fail || (second != nil) != tc.fail || len(data) != 1 || data[0] != s.shared.data {
			t.Fatalf("%s: installs %q into %q, errors %v and %v", tc.name, data, s.shared.data, first, second)
		}
		if got := environmentValue(s.m6Scope("m6-sample", "tools").m6ToolEnv(), envDataHome); got != s.shared.data {
			t.Fatalf("%s: a scope's tool environment names %q", tc.name, got)
		}
		if got := environmentValue(m6WithData([]string{envDataHome + "=/old", "HOME=/h"}, "/new"), envDataHome); got != "/new" {
			t.Fatalf("%s: m6WithData kept %q", tc.name, got)
		}
	}
}

package main

import (
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

// TestM6ToolsInstallSeesTheCallersRuntimes: the setup of the surface tools
// runs on the isolated prerequisite PATH plus the first caller directory
// whose node, and whose python3, is within the toolchain bounds, passing over
// an older runtime ahead of it, since the kit never installs either one.
func TestM6ToolsInstallSeesTheCallersRuntimes(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == goosWindows {
		t.Skip("the fake runtimes are shell scripts")
	}
	for _, tc := range []struct {
		name        string
		caller      []string
		want, avoid []string
	}{
		{"near usable runtimes behind old ones", []string{"old", "good"}, []string{"good"}, []string{"old"}},
		{"red only old runtimes", []string{"old"}, nil, []string{"old"}},
	} {
		s := newM6Suite(m2TestSuite(t))
		dir := m6FakeRuntimes(t, s, tc.caller)
		dirs := filepath.SplitList(m6SetupPath(t, s))
		m6RequireDirs(t, tc.name, dirs, tc.want, tc.avoid, dir)
		if len(dirs) == 0 || dirs[0] != s.m2.Bin {
			t.Errorf("%s: PATH %q no longer starts with the isolated directories", tc.name, dirs)
		}
		rows := filepath.SplitList(environmentValue(s.m6ToolEnv(), envPath))
		if len(tc.want) > 0 && (len(rows) == 0 || rows[0] != dir(tc.want[0])) {
			t.Errorf("%s: the tool rows run on PATH %q, want the setup runtimes first", tc.name, rows)
		}
	}
}

// m6RequireDirs requires dirs to hold every wanted runtime set and none of
// the avoided ones.
func m6RequireDirs(t *testing.T, name string, dirs, want, avoid []string, dir func(string) string) {
	t.Helper()
	for _, set := range want {
		if !slices.Contains(dirs, dir(set)) {
			t.Errorf("%s: PATH %q lacks the %s runtimes", name, dirs, set)
		}
	}
	for _, set := range avoid {
		if slices.Contains(dirs, dir(set)) {
			t.Errorf("%s: PATH %q holds the %s runtimes", name, dirs, set)
		}
	}
}

// m6FakeRuntimes writes an old and a good node and python3, puts the caller
// directories on the suite's PATH in order and answers where each set lives.
func m6FakeRuntimes(t *testing.T, s *m6Suite, caller []string) func(string) string {
	t.Helper()
	dir := func(name string) string { return filepath.Join(s.base, "runtimes", name) }
	for name, versions := range map[string][2]string{"old": {"v18.0.0", "Python 3.9.1"}, "good": {"v24.21.0", "Python 3.14.4"}} {
		for i, program := range []string{"node", "python3"} {
			script := "#!/bin/sh\necho '" + versions[i] + "'\n"
			if err := writeFile(filepath.Join(dir(name), program), []byte(script), execMode); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, name := range slices.Backward(caller) {
		s.env = prependPath(s.env, dir(name))
	}
	return dir
}

// m6SetupPath installs the tools through a scripted candidate and answers
// the PATH the setup command ran with.
func m6SetupPath(t *testing.T, s *m6Suite) string {
	t.Helper()
	var path string
	s.execute = m5Script(t, s.suite,
		m5Reply{prefix: []string{commandInit}, once: true, answer: m5Answer(result{})},
		m5Reply{prefix: []string{m6SetupCommand, m6SurfaceAll}, once: true, answer: func(_ *testing.T, c command) result {
			path = environmentValue(c.env, envPath)
			return result{}
		}},
	).Execute
	if err := s.m6Tools(t.Context()); err != nil {
		t.Fatalf("install = %v", err)
	}
	return path
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

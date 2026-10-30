package main

import (
	"fmt"
	"os"
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
		path := m6SetupPath(t, s)
		dirs := filepath.SplitList(path)
		m6RequireRuntimes(t, tc.name+": setup", path, tc.want, tc.avoid, dir)
		if len(dirs) == 0 || dirs[0] != s.m2.Bin {
			t.Errorf("%s: PATH %q no longer starts with the isolated directories", tc.name, dirs)
		}
		rows := environmentValue(s.m6ToolEnv(), envPath)
		m6RequireRuntimes(t, tc.name+": rows", rows, tc.want, tc.avoid, dir)
		if go1 := m6OnPath(rows, "go"); len(tc.want) > 0 && go1 != filepath.Join(dir("toolchain"), "go") {
			t.Errorf("%s: the tool rows find go at %q, want the caller's own", tc.name, go1)
		}
	}
}

// m6RequireRuntimes requires the node and python3 the PATH finds to be the
// wanted set's, and the go it finds never to come from a runtime set: the
// neighbors of a runtime, such as the runner's own older go, never shadow
// the caller's.
func m6RequireRuntimes(t *testing.T, name, path string, want, avoid []string, dir func(string) string) {
	t.Helper()
	for _, set := range want {
		for _, program := range []string{"node", "python3"} {
			got, err := filepath.EvalSymlinks(m6OnPath(path, program))
			wanted, wantErr := filepath.EvalSymlinks(filepath.Join(dir(set), program))
			if err != nil || wantErr != nil || got != wanted {
				t.Errorf("%s: %s resolves to %q, %v, want the %s runtimes", name, program, got, err, set)
			}
		}
	}
	for _, set := range slices.Concat(want, avoid) {
		if got := m6OnPath(path, "go"); got == filepath.Join(dir(set), "go") {
			t.Errorf("%s: go resolves to the %s runtimes' neighbor %q", name, set, got)
		}
	}
}

// m6OnPath is the first file named program along path, as a shell finds it.
func m6OnPath(path, program string) string {
	for _, dir := range filepath.SplitList(path) {
		if _, err := os.Lstat(filepath.Join(dir, program)); dir != "" && err == nil {
			return filepath.Join(dir, program)
		}
	}
	return ""
}

// m6FakeRuntimes writes an old and a good node and python3, each beside an
// older go as a runner's system directory holds one, puts the caller
// directories on the suite's PATH in order behind a toolchain directory of
// the caller's own go, and answers where each set lives.
func m6FakeRuntimes(t *testing.T, s *m6Suite, caller []string) func(string) string {
	t.Helper()
	dir := func(name string) string { return filepath.Join(s.base, "runtimes", name) }
	sets := map[string][3]string{"old": {"v18.0.0", "Python 3.9.1", "go version go1.26.10"}, "good": {"v24.21.0", "Python 3.14.4", "go version go1.26.10"}}
	sets["toolchain"] = [3]string{"", "", "go version go1.27.2"}
	for name, versions := range sets {
		for i, program := range []string{"node", "python3", "go"} {
			if versions[i] == "" {
				continue
			}
			script := "#!/bin/sh\necho '" + versions[i] + "'\n"
			if err := writeFile(filepath.Join(dir(name), program), []byte(script), execMode); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, name := range slices.Backward(slices.Concat([]string{"toolchain"}, caller)) {
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

// TestLinkRuntimeKeepsALinkAndNamesWhatItCannotLink: a second link of the
// same program keeps the first, a program the directory lacks is skipped,
// Windows gets a command shim that needs no privilege, and links that
// cannot be created name the cause.
func TestLinkRuntimeKeepsALinkAndNamesWhatItCannotLink(t *testing.T) {
	t.Parallel()
	dir, links := t.TempDir(), filepath.Join(t.TempDir(), runtimeLinksDir)
	checkedWrite(t, filepath.Join(dir, executable("node")), "node\n")
	for range 2 {
		if err := linkRuntime(dir, links, []string{"node", "npm"}, runtime.GOOS); err != nil {
			t.Fatalf("link = %v", err)
		}
	}
	if _, found := runtimeProgram(links, "npm", runtime.GOOS); found {
		t.Error("a program the runtime lacks was linked")
	}
	if _, found := runtimeProgram(links, "node", runtime.GOOS); !found {
		t.Error("the runtime's node was not linked")
	}
	windows, shims := t.TempDir(), t.TempDir()
	checkedWrite(t, filepath.Join(windows, "python.exe"), "MZ")
	if err := linkRuntime(windows, shims, []string{"python3", "python"}, goosWindows); err != nil {
		t.Fatalf("shim = %v", err)
	}
	shim, found := runtimeProgram(shims, "python", goosWindows)
	body, err := os.ReadFile(shim)
	if !found || err != nil || string(body) != "@\""+filepath.Join(windows, "python.exe")+"\" %*\r\n" {
		t.Errorf("the Windows shim = %q, %v, want one that starts the runtime's python.exe", body, err)
	}
	blocked := filepath.Join(t.TempDir(), "file")
	checkedWrite(t, blocked, "a file where the links go\n")
	if err := linkRuntime(dir, blocked, []string{"node"}, runtime.GOOS); err == nil {
		t.Error("links over a file were accepted")
	}
	locked := t.TempDir()
	if err := os.Chmod(locked, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(locked, dirMode); err != nil {
			t.Error(err)
		}
	})
	if err := linkRuntime(dir, locked, []string{"node"}, runtime.GOOS); runtime.GOOS != goosWindows && err == nil {
		t.Error("a link into a read-only directory was accepted")
	}
}

// TestSurfaceRuntimesNamesLinksItCannotWrite: a usable runtime whose links
// cannot be written fails the step that needs it, never a silent PATH.
func TestSurfaceRuntimesNamesLinksItCannotWrite(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == goosWindows {
		t.Skip("the fake runtimes are shell scripts")
	}
	s := newM6Suite(m2TestSuite(t))
	m6FakeRuntimes(t, s, []string{"good"})
	blocked := filepath.Join(t.TempDir(), "file")
	checkedWrite(t, blocked, "a file where the links go\n")
	if dirs, err := s.surfaceRuntimes(t.Context(), blocked); err == nil {
		t.Fatalf("links over a file = %q", dirs)
	}
}

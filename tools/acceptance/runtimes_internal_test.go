package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"unicode"
)

// TestRuntimeShimStartsATargetUnderASpacedNonASCIIPath: cmd.exe splits an
// unquoted path at its space and reads a batch file in the console code
// page, so the shim names its target quoted and by a path cmd reads whole,
// the short one where Windows keeps it.
func TestRuntimeShimStartsATargetUnderASpacedNonASCIIPath(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "acceptance área")
	target := filepath.Join(dir, "python.exe")
	checkedWrite(t, target, "MZ")
	shim := filepath.Join(dir, "links", "python.cmd")
	if err := os.MkdirAll(filepath.Dir(shim), dirMode); err != nil {
		t.Fatal(err)
	}
	if err := writeShim(strings.TrimSuffix(shim, ".cmd"), target); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(shim)
	if err != nil {
		t.Fatal(err)
	}
	named, found := runtimeShimTarget(string(body))
	if !found {
		t.Fatalf("shim = %q, want @\"<target>\" %%* and a CRLF", body)
	}
	if !sameFile(t, named, target) {
		t.Errorf("shim = %q names %s, want %s", body, named, target)
	}
	// Only Windows keeps a short name, whose path holds no space and no
	// rune a code page could read apart.
	if runtime.GOOS == goosWindows && shortPath(target) != target && strings.ContainsFunc(named, func(r rune) bool { return r == ' ' || r > unicode.MaxASCII }) {
		t.Errorf("shim = %q, want a target cmd reads whole in any code page", body)
	}
	if err := writeShim(filepath.Join(target, "python"), target); err == nil {
		t.Error("a shim under a file was accepted")
	}
	if runtime.GOOS == goosWindows {
		return
	}
	link := filepath.Join(dir, "links", "python3")
	if err := linkProgram(target, link); err != nil {
		t.Fatal(err)
	}
	if got, err := os.Readlink(link); err != nil || got != target {
		t.Errorf("link = %q, %v, want %s", got, err, target)
	}
}

// runtimeShimTarget is the target a shim starts: the line is the quoted
// target and the %* tail, ended by CRLF as cmd.exe writes a batch file.
func runtimeShimTarget(body string) (string, bool) {
	line, found := strings.CutSuffix(body, "\" %*\r\n")
	if !found {
		return "", false
	}
	target, found := strings.CutPrefix(line, "@\"")
	return target, found && target != "" && !strings.Contains(target, "\"")
}

func sameFile(t *testing.T, a, b string) bool {
	t.Helper()
	left, err := os.Stat(a)
	if err != nil {
		return false
	}
	right, err := os.Stat(b)
	return err == nil && os.SameFile(left, right)
}

// TestRuntimeShimRunsFromASpacedNonASCIIDirectory runs a shim the way the
// toolchain starts python.cmd: by the path PATH names, with an argument
// that holds a space and a non-ASCII rune.
func TestRuntimeShimRunsFromASpacedNonASCIIDirectory(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != goosWindows {
		t.Skip("cmd.exe starts the shim on Windows alone")
	}
	links := filepath.Join(t.TempDir(), "acceptance área", runtimeLinksDir)
	if err := os.MkdirAll(links, dirMode); err != nil {
		t.Fatal(err)
	}
	if err := writeShim(filepath.Join(links, "echoer"), os.Getenv("ComSpec")); err != nil {
		t.Fatal(err)
	}
	out, err := exec.CommandContext(t.Context(), filepath.Join(shortPath(links), "echoer.cmd"), "/c", "echo", "acceptance área").CombinedOutput()
	if err != nil || !strings.Contains(string(out), "acceptance") {
		t.Fatalf("shim run = %q, %v", out, err)
	}
}

// TestProgramCandidatesNameTheWindowsExtensions: Windows starts a program
// by its extension, every other system by its bare name.
func TestProgramCandidatesNameTheWindowsExtensions(t *testing.T) {
	t.Parallel()
	if got := programCandidates("node", true); !slices.Equal(got, []string{"node.exe", "node.cmd", "node.bat"}) {
		t.Errorf("windows candidates = %q", got)
	}
	if got := programCandidates("node", false); !slices.Equal(got, []string{"node"}) {
		t.Errorf("candidates = %q", got)
	}
}

// TestSurfaceRuntimesLinksTheUsableRuntimesOnEveryOS drives the selection
// with fake runtimes each system starts, a shell script or a batch file:
// the usable set is linked and named by the links directory alone, a PATH
// without one links nothing, and links that cannot be written fail.
func TestSurfaceRuntimesLinksTheUsableRuntimesOnEveryOS(t *testing.T) {
	t.Parallel()
	good := t.TempDir()
	fakeRuntime(t, good, "node", "v24.21.0")
	fakeRuntime(t, good, "python3", "Python 3.14.4")
	s := m2TestSuite(t)
	s.env = prependPath(s.env, good)
	links := filepath.Join(t.TempDir(), runtimeLinksDir)
	dirs, err := s.surfaceRuntimes(t.Context(), links)
	if err != nil || !slices.Equal(dirs, []string{shortPath(links)}) {
		t.Fatalf("surfaceRuntimes = %q, %v, want the links directory alone", dirs, err)
	}
	for _, name := range []string{"node", "python3"} {
		if _, found := runtimeProgram(links, name); !found {
			t.Errorf("%s was not linked", name)
		}
	}
	blocked := filepath.Join(t.TempDir(), "file")
	checkedWrite(t, blocked, "a file where the links go\n")
	if dirs, err := s.surfaceRuntimes(t.Context(), blocked); err == nil {
		t.Errorf("links over a file = %q", dirs)
	}
	bare := m2TestSuite(t)
	bare.env = windowsPathM2(bare.env, []string{t.TempDir()})
	if dirs, err := bare.surfaceRuntimes(t.Context(), filepath.Join(t.TempDir(), runtimeLinksDir)); err != nil || dirs != nil {
		t.Errorf("no usable runtime = %q, %v, want nothing linked", dirs, err)
	}
}

// fakeRuntime writes a program that prints version, as a batch file on
// Windows and a shell script elsewhere.
func fakeRuntime(t *testing.T, dir, name, version string) {
	t.Helper()
	if runtime.GOOS == goosWindows {
		checkedWrite(t, filepath.Join(dir, name+".cmd"), "@echo "+version+"\r\n")
		return
	}
	if err := writeFile(filepath.Join(dir, name), []byte("#!/bin/sh\necho '"+version+"'\n"), execMode); err != nil {
		t.Fatal(err)
	}
}

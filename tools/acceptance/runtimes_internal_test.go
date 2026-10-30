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
	if err := linkProgram(target, strings.TrimSuffix(shim, ".cmd"), true); err != nil {
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
	if err := linkProgram(target, filepath.Join(target, "python"), true); err == nil {
		t.Error("a shim under a file was accepted")
	}
	if runtime.GOOS == goosWindows {
		return
	}
	link := filepath.Join(dir, "links", "python3")
	if err := linkProgram(target, link, false); err != nil {
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
	if err := linkProgram(os.Getenv("ComSpec"), filepath.Join(links, "echoer"), true); err != nil {
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

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
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
	if err != nil || string(body) != "@\""+shortPath(target)+"\" %*\r\n" {
		t.Fatalf("shim = %q, %v, want the quoted short path of %s", body, err, target)
	}
	if runtime.GOOS == goosWindows && shortPath(target) != target && bytes.ContainsAny(body, " á") {
		t.Errorf("shim = %q, want a path cmd reads in any code page", body)
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

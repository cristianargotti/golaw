package toolchain

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestLookPathReadsOnlyTheEnvironmentItIsGiven(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	plain := filepath.Join(dir, "tool")
	if err := os.WriteFile(plain, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, found := lookPath("tool", []string{"PATH=" + dir}, "linux"); found {
		t.Error("a file without an executable bit was found off Windows")
	}
	shim := filepath.Join(dir, "tool.cmd")
	if err := os.WriteFile(shim, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if got, found := lookPath("tool", []string{"Path=" + filepath.Join(dir, "none"), "Path=" + string(filepath.ListSeparator) + dir}, goosWindows); !found || got != shim {
		t.Errorf("lookPath on windows = %s, %t, want the .cmd before the bare name, from the last Path", got, found)
	}
	if _, found := lookPath("tool", nil, goosWindows); found {
		t.Error("an environment without PATH found a tool")
	}
	if err := os.Mkdir(filepath.Join(dir, "sub.exe"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, found := lookPath("sub", []string{"PATH=" + dir}, goosWindows); found {
		t.Error("a directory was found as an executable")
	}
}

func TestRuntimeExecutablesFollowTheOS(t *testing.T) {
	t.Parallel()
	if got := runtimeExecutables(runtimePython, goosWindows); !slices.Equal(got, []string{"python", "python3"}) {
		t.Errorf("windows python = %v", got)
	}
	if got := runtimeExecutables(runtimeNode, goosWindows); !slices.Equal(got, []string{"node"}) {
		t.Errorf("windows node = %v", got)
	}
}

func TestTrimZerosKeepsAMinorThatEndsInZero(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{"22.13.0": "22.13", "3.10": "3.10", "3.0.0": "3.0", "22.13.1": "22.13.1"} {
		if got := trimZeros(in); got != want {
			t.Errorf("trimZeros(%s) = %s, want %s", in, got, want)
		}
	}
}

func TestRuntimeOfABinaryToolIsNone(t *testing.T) {
	t.Parallel()
	if _, ok := runtimeOf(Tool{Name: "x", Distribution: DistBinary}); ok {
		t.Error("a binary tool needs a runtime")
	}
}

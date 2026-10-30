package toolchain_test

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/runnertest"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

// TestExecStartsAScriptWhoseInterpreterPathHoldsASpace: a virtual
// environment under a home with a space writes a plain script whose #! line
// the kernel splits at that space; Exec starts its interpreter with the
// script as the first argument, and a script with no space starts as it is.
func TestExecStartsAScriptWhoseInterpreterPathHoldsASpace(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Windows starts no script through a #! line")
	}
	for _, tc := range []struct{ name, dir string }{
		{"red interpreter under a spaced home", "First Last"},
		{"near interpreter under a plain home", "plain"},
	} {
		bin, script := fakeVenvScript(t, tc.dir)
		got, err := toolchain.Exec(t.Context(), toolchain.Request{Path: script, Args: []string{"--version"}, Dir: bin, Timeout: 30 * time.Second})
		if tc.dir == "plain" {
			if err != nil || got.Output.Code != 0 || string(got.Output.Stdout) != "checkov 3.3.30\n" {
				t.Errorf("%s: Exec = %+v, %v", tc.name, got, err)
			}
			continue
		}
		if err != nil || got.Output.Code != 0 || string(got.Output.Stdout) != "checkov 3.3.30\n" {
			t.Fatalf("%s: Exec = %+v, %v", tc.name, got, err)
		}
		if calls := runnertest.Calls(t, bin, "python"); len(calls) != 1 || !slices.Equal(calls[0], []string{script, "--version"}) {
			t.Errorf("%s: interpreter calls = %q, want the script then its arguments", tc.name, calls)
		}
	}
}

// fakeVenvScript writes a fake interpreter under dir/venv/bin and a plain
// script beside it whose #! line names that interpreter.
func fakeVenvScript(t *testing.T, dir string) (string, string) {
	t.Helper()
	bin := filepath.Join(t.TempDir(), dir, "venv", "bin")
	if err := os.MkdirAll(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	python := runnertest.Install(t, bin, "python", runnertest.Behavior{Stdout: "checkov 3.3.30\n"})
	script := filepath.Join(bin, "checkov")
	if err := os.WriteFile(script, []byte("#!"+python+"\nfrom checkov.main import Checkov\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return bin, script
}

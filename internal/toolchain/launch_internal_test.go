package toolchain

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestLaunchStartsEveryScriptThroughItsInterpreterOnWindows: Windows never
// reads a #! line, so the plain script pip copies for checkov starts through
// the interpreter that line names, quoted or not; elsewhere the kernel reads
// the line and only a spaced interpreter needs the kit.
func TestLaunchStartsEveryScriptThroughItsInterpreterOnWindows(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	python := filepath.Join(dir, "python.exe")
	writeLaunchFile(t, python, "MZ")
	plain := filepath.Join(dir, "checkov")
	writeLaunchFile(t, plain, "#!"+python+"\r\nfrom checkov.main import Checkov\r\n")
	quoted := filepath.Join(dir, "quoted")
	writeLaunchFile(t, quoted, "#!\""+python+"\"\r\n")
	exe := filepath.Join(dir, "yamllint.exe")
	writeLaunchFile(t, exe, "MZ")
	for _, tc := range []struct {
		name, path, goos, program string
		args                      []string
	}{
		{"red a plain script on windows", plain, goosWindows, python, []string{plain, "--version"}},
		{"red a quoted interpreter on windows", quoted, goosWindows, python, []string{quoted, "--version"}},
		{"near an executable on windows", exe, goosWindows, exe, []string{"--version"}},
		{"near a plain script the kernel starts", plain, "linux", plain, []string{"--version"}},
	} {
		program, args := launch(tc.path, []string{"--version"}, tc.goos)
		if program != tc.program || !slices.Equal(args, tc.args) {
			t.Errorf("%s: launch = %s %q, want %s %q", tc.name, program, args, tc.program, tc.args)
		}
	}
}

func writeLaunchFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

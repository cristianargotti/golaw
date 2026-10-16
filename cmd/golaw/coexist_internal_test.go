package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook/coexist"
)

// gateHome is a home whose settings register the gate on writes and whose
// registry governs the repository.
func gateHome(t *testing.T, repo string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Dir(coexist.SettingsPath(dir)), 0o750); err != nil {
		t.Fatal(err)
	}
	settings := `{"hooks": {"PreToolUse": [{"matcher": "Write|Edit", "hooks": [{"command": "python3 pcc_gate.py"}]}]}}`
	registry := `{"repos": ["` + filepath.ToSlash(repo) + `"]}`
	for path, content := range map[string]string{coexist.SettingsPath(dir): settings, coexist.RegistryPath(dir): registry} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestCoexistRefusesWhileTheGateOwnsAFamily(t *testing.T) {
	t.Parallel()
	ta := newTestApp(t)
	repo := t.TempDir()
	h := gateHome(t, repo)
	ta.home = func() (string, error) { return h, nil }
	if got := ta.run(t.Context(), []string{"coexist", repo}); got != exitFindings {
		t.Fatalf("exit = %d, stderr = %q", got, ta.stderr.String())
	}
	out := ta.stdout.String()
	for _, want := range []string{"gate: registered for write; governs this repository\n", "write       text         python-gate\n", "read        secret-read  kit\n", "refused: the Python gate still owns a blocking verdict: text (switch text_checks off), paths (switch paths_checks off)\n"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %q", want, out)
		}
	}
}

func TestCoexistIsCleanWithoutAGate(t *testing.T) {
	t.Parallel()
	ta := newTestApp(t)
	if got := ta.run(t.Context(), []string{"coexist"}); got != exitClean {
		t.Fatalf("exit = %d, stderr = %q", got, ta.stderr.String())
	}
	if !strings.HasPrefix(ta.stdout.String(), "gate: no hook registered on this machine\n") {
		t.Fatalf("stdout = %q", ta.stdout.String())
	}
}

func TestCoexistReportsFailures(t *testing.T) {
	t.Parallel()
	ta := newTestApp(t)
	ta.home = func() (string, error) { return "", errBoom }
	if got := ta.run(t.Context(), []string{"coexist"}); got != exitFailure || ta.stderr.String() != "golaw: coexist: boom\n" {
		t.Fatalf("exit = %d, stderr = %q", got, ta.stderr.String())
	}
	broken := newTestApp(t)
	h := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(coexist.SettingsPath(h)), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(coexist.SettingsPath(h), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	broken.home = func() (string, error) { return h, nil }
	if got := broken.run(t.Context(), []string{"coexist"}); got != exitFailure || !strings.Contains(broken.stderr.String(), "cannot read the gate registration") {
		t.Fatalf("exit = %d, stderr = %q", got, broken.stderr.String())
	}
}

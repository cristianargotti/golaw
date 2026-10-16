package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook/coexist"
)

// gateHome is a home whose settings, in the default configuration
// directory of the agent, register the gate on writes and whose registry
// governs the repository.
func gateHome(t *testing.T, repo string) string {
	t.Helper()
	dir := t.TempDir()
	writeGate(t, defaultConfig(dir), repo)
	return dir
}

// defaultConfig is the configuration directory of the agent under a home
// without an override.
func defaultConfig(home string) string {
	return agentUnderTest().ConfigDir(func(string) string { return "" }, home)
}

// writeGate registers the gate on writes in a configuration directory and
// governs the repository from the registry beside the settings.
func writeGate(t *testing.T, config, repo string) {
	t.Helper()
	if err := os.MkdirAll(config, 0o750); err != nil {
		t.Fatal(err)
	}
	settings := `{"hooks": {"PreToolUse": [{"matcher": "Write|Edit", "hooks": [{"command": "python3 pcc_gate.py"}]}]}}`
	registry := `{"repos": ["` + filepath.ToSlash(repo) + `"]}`
	for path, content := range map[string]string{coexist.SettingsPath(config): settings, coexist.RegistryPath(config): registry} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
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
	if err := os.MkdirAll(defaultConfig(h), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(coexist.SettingsPath(defaultConfig(h)), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	broken.home = func() (string, error) { return h, nil }
	if got := broken.run(t.Context(), []string{"coexist"}); got != exitFailure || !strings.Contains(broken.stderr.String(), "cannot read the gate registration") {
		t.Fatalf("exit = %d, stderr = %q", got, broken.stderr.String())
	}
}

// TestAgentInstallFindsTheGateUnderTheConfigOverride registers the gate in
// the directory the agent's override names, the settings agent install
// writes, so the refusal of A17 reads the file the host reads.
func TestAgentInstallFindsTheGateUnderTheConfigOverride(t *testing.T) {
	t.Parallel()
	ta, port, root := agentApp(t)
	override := filepath.Join(t.TempDir(), "override")
	writeGate(t, override, root)
	ta.env = func(string) string { return override }
	if got := ta.run(t.Context(), []string{"agent", "install", "--agent", agentUnderTest().ID, root}); got != exitFindings {
		t.Fatalf("exit = %d, stderr = %q", got, ta.stderr.String())
	}
	if !strings.Contains(ta.stdout.String(), "refused: ") || port.calls != 0 {
		t.Fatalf("stdout = %q, port = %+v", ta.stdout.String(), port)
	}
	coexistApp := newTestApp(t)
	coexistApp.env = ta.env
	if got := coexistApp.run(t.Context(), []string{"coexist", root}); got != exitFindings {
		t.Fatalf("coexist exit = %d, stderr = %q", got, coexistApp.stderr.String())
	}
}

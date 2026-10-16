package coexist_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook/coexist"
)

// gateSettings registers the gate on every event, beside a foreign hook.
const gateSettings = `{
  "hooks": {
    "PreToolUse": [
      {"matcher": "Bash", "hooks": [{"type": "command", "command": "python3 /opt/hooks/classify.py"}]},
      {"matcher": "Write|Edit|MultiEdit", "hooks": [{"type": "command", "command": "python3 /opt/gate/scripts/pcc_gate.py", "timeout": 20}]},
      {"matcher": "Bash", "hooks": [{"type": "command", "command": "python3 /opt/gate/scripts/pcc_gate.py"}]},
      {"matcher": "Read", "hooks": [{"type": "command", "command": "python3 /opt/gate/scripts/pcc_gate.py"}]}
    ],
    "PostToolUse": [
      {"matcher": "Write|Edit|MultiEdit", "hooks": [{"type": "command", "command": "python3 /opt/gate/scripts/pcc_gate.py"}]},
      {"matcher": "Bash", "hooks": [{"type": "command", "command": "python3 /opt/gate/scripts/pcc_gate.py"}]}
    ],
    "SessionStart": [
      {"matcher": "startup", "hooks": [{"type": "command", "command": "python3 /opt/gate/scripts/pcc_gate.py"}]}
    ]
  }
}`

// home writes the two registration files under a temporary home and
// returns it; an empty content skips the file.
func home(t *testing.T, settings, registry string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".claude"), 0o750); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{"settings.json": settings, "pcc-standards.json": registry} {
		if content == "" {
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, ".claude", name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// registryFor lists the repository as governed, with the given extra keys.
func registryFor(repo, extra string) string {
	return `{"repos": ["` + filepath.ToSlash(repo) + `", "/elsewhere/other"], "max_file_lines": 300` + extra + `}`
}

func TestDetectWithoutRegistrationIsNoGate(t *testing.T) {
	t.Parallel()
	d, err := detect(home(t, "", ""), t.TempDir())
	if err != nil || d.Governs || len(d.Registered) != 0 || len(d.Released) != 0 {
		t.Fatalf("detection = %+v, %v", d, err)
	}
	table := coexist.Build(d)
	if err := table.Refuse(); err != nil {
		t.Fatalf("refuse = %v", err)
	}
	if got := table.Lines()[0]; got != "gate: no hook registered on this machine" {
		t.Fatalf("summary = %q", got)
	}
}

func TestDetectReadsEventsRepositoriesAndFlags(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	deep := filepath.Join(repo, "internal", "deep")
	if err := os.MkdirAll(deep, 0o750); err != nil {
		t.Fatal(err)
	}
	h := home(t, gateSettings, registryFor(repo, `, "text_checks": false, "paths_checks": false, "shell_checks": true`))
	d, err := detect(h, deep)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range []string{"write", "read", "shell", "post-write", "post-shell"} {
		if !d.Registered[event] {
			t.Fatalf("event %s not registered: %+v", event, d.Registered)
		}
	}
	if !d.Governs || !d.Released["text"] || !d.Released["paths"] || d.Released["shell"] || len(d.Released) != 2 {
		t.Fatalf("detection = %+v", d)
	}
}

func TestDetectDoesNotGovernAnotherRepository(t *testing.T) {
	t.Parallel()
	h := home(t, gateSettings, registryFor(t.TempDir(), ""))
	d, err := detect(h, t.TempDir())
	if err != nil || d.Governs {
		t.Fatalf("detection = %+v, %v", d, err)
	}
	if err := coexist.Build(d).Refuse(); err != nil {
		t.Fatalf("refuse = %v", err)
	}
}

func TestDetectRefusesAMalformedRegistration(t *testing.T) {
	t.Parallel()
	for name, files := range map[string][2]string{"settings": {"{", ""}, "registry": {gateSettings, "[]"}} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := detect(home(t, files[0], files[1]), t.TempDir()); !errors.Is(err, coexist.ErrDetect) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestBuildAssignsOwnersAndRefuseNamesTheFlags(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	h := home(t, gateSettings, registryFor(repo, `, "sizes_checks": false`))
	d, err := detect(h, repo)
	if err != nil {
		t.Fatal(err)
	}
	table := coexist.Build(d)
	if got := table.Owner("write", "text"); got != coexist.OwnerGate {
		t.Fatalf("write text = %s", got)
	}
	if got := table.Owner("write", "sizes"); got != coexist.OwnerKit {
		t.Fatalf("write sizes = %s", got)
	}
	if got := table.Owner("commit", "sizes"); got != coexist.OwnerKit {
		t.Fatalf("unknown row = %s", got)
	}
	err = table.Refuse()
	if !errors.Is(err, coexist.ErrOwnedElsewhere) {
		t.Fatalf("refuse = %v", err)
	}
	want := "text (switch text_checks off), paths (switch paths_checks off), secret-read (switch secret_read_checks off), shell (switch shell_checks off)"
	if !strings.HasSuffix(err.Error(), want) {
		t.Fatalf("refuse = %q", err.Error())
	}
	lines := table.Lines()
	if lines[0] != "gate: registered for write, read, shell, post-write, post-shell; governs this repository" {
		t.Fatalf("summary = %q", lines[0])
	}
	if len(lines) != 11 || !strings.HasPrefix(lines[2], "write       text         python-gate") {
		t.Fatalf("lines = %q", lines)
	}
}

func TestGateWithoutTheEventDoesNotOwnTheFamily(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	readOnly := `{"hooks": {"PreToolUse": [{"matcher": "Read", "hooks": [{"command": "python3 pcc_gate.py"}]}]}}`
	d, err := detect(home(t, readOnly, registryFor(repo, "")), repo)
	if err != nil {
		t.Fatal(err)
	}
	table := coexist.Build(d)
	if table.Owner("read", "secret-read") != coexist.OwnerGate || table.Owner("write", "text") != coexist.OwnerKit {
		t.Fatalf("rows = %+v", table.Rows)
	}
	if got := coexist.Owner(0).String(); got != "unknown" {
		t.Fatalf("zero owner = %q", got)
	}
}

func TestDetectResolvesARelativeRootAndAHomeEntry(t *testing.T) {
	t.Parallel()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		repo, root string
		governs    bool
	}{
		"dot":            {filepath.ToSlash(cwd), ".", true},
		"relative child": {filepath.ToSlash(filepath.Dir(cwd)), filepath.Join("..", "coexist"), true},
		"home entry":     {"~/repo", "", true},
		"elsewhere":      {filepath.ToSlash(t.TempDir()), ".", false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := home(t, "", `{"repos": ["`+tc.repo+`"]}`)
			root := tc.root
			if root == "" {
				root = filepath.Join(h, "repo")
				if err := os.Mkdir(root, 0o750); err != nil {
					t.Fatal(err)
				}
			}
			if d, err := detect(h, root); err != nil || d.Governs != tc.governs {
				t.Fatalf("detection = %+v, %v", d, err)
			}
		})
	}
}

// TestDetectReadsTheConfigDirectoryOfTheAgent finds the gate where the
// host reads its settings, a directory the agent's override names, and
// ignores a registration left in the default directory of the home.
func TestDetectReadsTheConfigDirectoryOfTheAgent(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	h := home(t, gateSettings, registryFor(repo, ""))
	config := filepath.Join(t.TempDir(), "agent config")
	if err := os.Mkdir(config, 0o750); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"settings.json", "pcc-standards.json"} {
		data, err := os.ReadFile(filepath.Join(h, ".claude", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(config, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if d, err := coexist.Detect(t.TempDir(), config, repo); err != nil || !d.Governs || len(d.Registered) == 0 {
		t.Fatalf("the override = %+v, %v", d, err)
	}
	if d, err := coexist.Detect(h, filepath.Join(t.TempDir(), "empty"), repo); err != nil || d.Governs || len(d.Registered) != 0 {
		t.Fatalf("the default directory read under an override = %+v, %v", d, err)
	}
	if got := coexist.SettingsPath(config); got != filepath.Join(config, "settings.json") {
		t.Fatalf("settings path = %s", got)
	}
}

// detect reads the registration from the default configuration directory
// of the home, where the host keeps its settings without an override.
func detect(home, root string) (coexist.Detection, error) {
	d, err := coexist.Detect(home, filepath.Join(home, ".claude"), root)
	if err != nil {
		return d, fmt.Errorf("detect: %w", err)
	}
	return d, nil
}

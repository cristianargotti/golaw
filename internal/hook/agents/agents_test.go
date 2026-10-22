package agents_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook/agents"
	"github.com/cristianargotti/golaw/internal/install"
)

func TestAllListsTheThreeAgentsComplete(t *testing.T) {
	t.Parallel()
	all := agents.All()
	if got := agents.IDs(); !slices.Equal(got, []string{"claude", "codex", "cursor"}) || len(all) != 3 || all[0].ID != "claude" {
		t.Fatalf("ids = %v", got)
	}
	for _, a := range all {
		if a.Adapter.Parse == nil || a.Adapter.Encode == nil || a.Adapter.Oversized == nil ||
			a.Plan == nil || a.Uninstall == nil || a.Doctor == nil || a.ConfigDir == nil {
			t.Fatalf("%s is incomplete", a.ID)
		}
		if got, ok := agents.Lookup(a.ID); !ok || got.ID != a.ID {
			t.Fatalf("lookup %s = %+v, %v", a.ID, got, ok)
		}
	}
	if _, ok := agents.Lookup("nobody"); ok {
		t.Fatal("an unknown id was found")
	}
}

func TestConfigDirsOfTheHooksFileAgents(t *testing.T) {
	t.Parallel()
	codex, _ := agents.Lookup("codex")
	cursor, _ := agents.Lookup("cursor")
	override := func(name string) string {
		if name == "CODEX_HOME" {
			return filepath.Join("codex", "home")
		}
		return ""
	}
	none := func(string) string { return "" }
	rows := []struct {
		name, got, want string
	}{
		{"codex override", codex.ConfigDir(override, "home"), filepath.Join("codex", "home")},
		{"codex fallback", codex.ConfigDir(none, "home"), filepath.Join("home", ".codex")},
		{"cursor ignores the codex override", cursor.ConfigDir(override, "home"), filepath.Join("home", ".cursor")},
	}
	for _, row := range rows {
		if row.got != row.want {
			t.Errorf("%s = %q, want %q", row.name, row.got, row.want)
		}
	}
}

func TestConfigDirHonorsTheHostOverride(t *testing.T) {
	t.Parallel()
	a, _ := agents.Lookup("claude")
	override := func(name string) string {
		if name == "CLAUDE_CONFIG_DIR" {
			return filepath.Join("over", "ride")
		}
		return ""
	}
	if got := a.ConfigDir(override, "home"); got != filepath.Join("over", "ride") {
		t.Fatalf("override = %q", got)
	}
	if got := a.ConfigDir(func(string) string { return "" }, "home"); got != filepath.Join("home", ".claude") {
		t.Fatalf("fallback = %q", got)
	}
}

func TestReceiptPathMatchesTheInstallerLayout(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	root := filepath.Join(base, "repository")
	if err := os.MkdirAll(root, 0o750); err != nil {
		t.Fatal(err)
	}
	_, identity, err := install.ProjectIdentity(root)
	if err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(base, "state")
	for _, scope := range []string{"user", "project"} {
		plan, err := install.PlanClaude(install.Options{
			Root: root, ConfigDir: filepath.Join(base, "config"), StateDir: state,
			Scope: scope, Version: "0.1.0", PolicyDigest: "policy",
		})
		if err != nil {
			t.Fatal(err)
		}
		if want := agents.ReceiptPath(state, "claude", scope, identity); plan.ReceiptPath != want {
			t.Errorf("%s receipt = %q, want %q", scope, plan.ReceiptPath, want)
		}
	}
	want := filepath.Join(state, "claude", "observations", identity+".json")
	if got := agents.ObservationPath(state, "claude", identity); got != want {
		t.Errorf("observation = %q, want %q", got, want)
	}
}

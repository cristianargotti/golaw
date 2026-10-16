package agents_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook/agents"
	"github.com/cristianargotti/golaw/internal/install"
)

func TestAllListsOneCompleteAgent(t *testing.T) {
	t.Parallel()
	all := agents.All()
	if len(all) != 1 || all[0].ID != "claude" {
		t.Fatalf("agents = %+v", all)
	}
	a := all[0]
	if a.Adapter.Header == nil || a.Adapter.Decode == nil || a.Adapter.Encode == nil || a.Plan == nil || a.ConfigDir == nil {
		t.Fatal("the agent is incomplete")
	}
	if got := agents.IDs(); !slices.Equal(got, []string{"claude"}) {
		t.Fatalf("ids = %v", got)
	}
	if _, ok := agents.Lookup("nobody"); ok {
		t.Fatal("an unknown id was found")
	}
	if got, ok := agents.Lookup("claude"); !ok || got.ID != "claude" {
		t.Fatalf("lookup = %+v, %v", got, ok)
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

package agents_test

import (
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/hook/agents"
)

func TestCapturePathSitsBesideTheObservations(t *testing.T) {
	t.Parallel()
	state := t.TempDir()
	want := filepath.Join(state, "cursor", "captures", "identity.json")
	if got := agents.CapturePath(state, "cursor", "identity"); got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestHookAssetNamesTheDefinitionOfEachAgent(t *testing.T) {
	t.Parallel()
	rows := map[string]string{
		"claude": "agents/claude/hooks/hooks.json",
		"codex":  "agents/codex/hooks.json",
		"cursor": "agents/cursor/hooks.json",
	}
	for id, want := range rows {
		if got := agents.HookAsset(id); got != want {
			t.Errorf("%s: %s, want %s", id, got, want)
		}
	}
	for _, a := range agents.All() {
		if _, err := assets.Read(agents.HookAsset(a.ID)); err != nil {
			t.Errorf("%s: %v", a.ID, err)
		}
	}
}

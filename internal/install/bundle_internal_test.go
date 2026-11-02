package install

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
)

func TestContentVersionFollowsThePluginFilesOnly(t *testing.T) {
	t.Parallel()
	files := []assets.File{
		{Path: "agents/claude/skills/golaw/SKILL.md", Data: []byte("skill\n")},
		{Path: pluginManifest, Data: []byte(`{"version": "0.1.0"}`)},
		{Path: "agents/codex/AGENTS.md", Data: []byte("codex\n")},
	}
	base := contentVersion(files)
	if !strings.HasPrefix(base, "g") || len(base) != 13 {
		t.Fatalf("content version = %q, want g and 12 hex digits", base)
	}
	files[1].Data = []byte(`{"version": "9.9.9"}`)
	files[2].Data = []byte("another codex\n")
	if got := contentVersion(files); got != base {
		t.Errorf("the manifest and other agents moved the version: %q, want %q", got, base)
	}
	files[0].Data = []byte("a new skill\n")
	if got := contentVersion(files); got == base {
		t.Error("a new skill kept the version, so the host would keep the old copy")
	}
}

func TestBundleDataVersionsThePluginAndNamesTheMarketplace(t *testing.T) {
	t.Parallel()
	plugin, err := bundleData(assets.File{Path: pluginManifest, Data: []byte(`{"name": "golaw", "version": "0.1.0"}`)}, "gabc", "golaw-x")
	if err != nil {
		t.Fatal(err)
	}
	market, err := bundleData(assets.File{Path: marketplaceManifest, Data: []byte(`{"name": "golaw", "plugins": []}`)}, "gabc", "golaw-x")
	if err != nil {
		t.Fatal(err)
	}
	var p, m map[string]any
	if json.Unmarshal(plugin, &p) != nil || json.Unmarshal(market, &m) != nil {
		t.Fatalf("manifests = %s %s", plugin, market)
	}
	if p["name"] != "golaw" || p["version"] != "0.1.0-gabc" || m["name"] != "golaw-x" || m["plugins"] == nil || !strings.HasSuffix(string(plugin), "}\n") {
		t.Errorf("manifests = %s %s", plugin, market)
	}
	skill := []byte("skill\n")
	if got, err := bundleData(assets.File{Path: "agents/claude/skills/golaw/SKILL.md", Data: skill}, "gabc", "golaw-x"); err != nil || !bytes.Equal(got, skill) {
		t.Errorf("another asset = %q, %v", got, err)
	}
	for name, data := range map[string]string{"not json": "{", "no version": `{"name": "golaw"}`} {
		if _, err := bundleData(assets.File{Path: pluginManifest, Data: []byte(data)}, "gabc", "golaw-x"); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func TestMarketplaceNameIsPerProject(t *testing.T) {
	t.Parallel()
	if got := MarketplaceName("user", "0123456789abcdef"); got != "golaw" {
		t.Errorf("user = %q", got)
	}
	if got := MarketplaceName(scopeProject, "0123456789abcdef"); got != "golaw-0123456789ab" {
		t.Errorf("project = %q", got)
	}
}

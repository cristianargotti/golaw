package doctor

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/model"
)

func baseline(t *testing.T) Snapshot {
	t.Helper()
	hooks, err := assets.Read("agents/claude/hooks/hooks.json")
	if err != nil {
		t.Fatal(err)
	}
	return Snapshot{
		Receipt: install.Receipt{
			Schema: 1, ProjectID: "repository", Version: "0.1.0",
			Artifacts:      []install.Artifact{{Path: "asset", SHA256: install.Digest([]byte("ok"))}},
			HookDefinition: hooks,
		},
		Files:    map[string]string{"asset": "ok"},
		Settings: `{"enabledPlugins":{"golaw@golaw":true},"permissions":{"deny":["Read(**/*.key)","Read(notes/*)"]}}`,
		Hooks:    string(hooks), RequiredRead: []string{"Read(**/*.key)"},
		Plugin: `{"name":"golaw","version":"0.1.0"}`,
		Binary: BinaryEvidence{
			Source: "agent-observed", HookPath: "bin/golaw", RunningPath: "bin/golaw",
		},
		ClaudeVersion: "2.1.295 (Claude Code)", PinnedVersion: "0.1.0",
		Build:      BuildMetadata{Version: "v0.1.0", Sum: "h1:fixture", Revision: "revision"},
		Enrollment: model.Enrollment{State: model.EnrollmentEnrolled, PolicyDigest: "policy"},
	}
}

func patched(base Snapshot, data []byte) (Snapshot, error) {
	encoded, err := json.Marshal(base)
	if err != nil {
		return Snapshot{}, fmt.Errorf("encode baseline fixture: %w", err)
	}
	var out Snapshot
	if err := json.Unmarshal(encoded, &out); err != nil {
		return Snapshot{}, fmt.Errorf("decode baseline fixture: %w", err)
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return Snapshot{}, fmt.Errorf("apply diagnostic fixture: %w", err)
	}
	return out, nil
}

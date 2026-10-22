package install

import (
	"strings"
	"testing"
)

// A group another tool inserted before golaw's after the install shifts
// golaw's trust identity: the next install names the old and the new key
// and the /hooks step in the human channel (section 5.6).
func TestCodexInstallReportsAMovedTrustIdentity(t *testing.T) {
	t.Parallel()
	options := codexOptions(t, "user")
	installed := codexInstall(t, options)
	document := codexDecode(t, cinRead(t, installed.Receipt.HooksPath))
	events, ok := document["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("hooks is not a mapping: %T", document["hooks"])
	}
	groups, ok := events["PreToolUse"].([]any)
	if !ok {
		t.Fatalf("PreToolUse is not a group list: %T", events["PreToolUse"])
	}
	foreign := codexDecode(t, []byte(`{"matcher":"Bash","hooks":[{"type":"command","command":"other kit","timeout":5}]}`))
	events["PreToolUse"] = append([]any{foreign}, groups...)
	cinWrite(t, installed.Receipt.HooksPath, codexJSON(t, document))
	plan, err := PlanCodex(options)
	must(t, err)
	notes := strings.Join(plan.Notes, "\n")
	old := CodexTrustKey(installed.Receipt.HooksPath, "PreToolUse", 0, 0)
	moved := CodexTrustKey(installed.Receipt.HooksPath, "PreToolUse", 1, 0)
	if !strings.Contains(notes, old+" mudou para "+moved) || !strings.Contains(notes, "/hooks") {
		t.Fatalf("notes = %q", notes)
	}
	if !installed.Receipt.HooksCreated || !plan.Receipt.HooksCreated {
		t.Fatalf("a hooks file the install created = %+v", plan.Receipt)
	}
}

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// TestHookAsksToCommitAPolicyGrownPastItsBound pins the stated limit of a
// golaw.yml grown past the bound of the checkout after init enrolled it,
// through the real binding: before its first commit the hook does not read
// it from the checkout and denies with the reason that asks for the commit,
// and once git holds it, staged, the hook takes it.
func TestHookAsksToCommitAPolicyGrownPastItsBound(t *testing.T) {
	t.Parallel()
	e := enrolledBeforeCommit(t, false)
	agent := agentUnderTest()
	file := filepath.Join(e.root, "golaw.yml")
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, strings.Repeat("# padding\n", model.MaxPolicyBytes/10+1)...)
	if err := os.WriteFile(file, data, 0o600); err != nil {
		t.Fatal(err)
	}
	write := func() string {
		event := eventAt(t, "Write", e.root, map[string]any{"file_path": filepath.Join(e.root, "notes.md"), "content": "one-two\n"})
		return string(hook.Run(t.Context(), agent.ID, strings.NewReader(event), agent.Adapter, e.deps).Stdout)
	}
	if said := write(); !strings.Contains(said, `"permissionDecision":"deny"`) || !strings.Contains(said, "ask the person to commit golaw.yml") {
		t.Fatalf("a policy past the bound in the checkout: %s", said)
	}
	gitRun(t, e.root, "add", "golaw.yml")
	if said := write(); said != "" {
		t.Fatalf("the same policy staged: %s", said)
	}
}

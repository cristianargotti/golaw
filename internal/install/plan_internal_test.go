package install

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestPlanDoesNotWrite(t *testing.T) {
	t.Parallel()
	options := testOptions(t)
	original, err := os.ReadFile("testdata/foreign.json")
	must(t, err)
	settings := filepath.Join(options.ConfigDir, "settings.json")
	must(t, os.WriteFile(settings, original, 0o600))
	plan, err := PlanClaude(options)
	must(t, err)
	current, err := os.ReadFile(settings)
	must(t, err)
	if !bytes.Equal(current, original) || len(plan.Changes) == 0 {
		t.Fatal("planning wrote files or omitted its diff")
	}
	if _, err := os.Stat(plan.ReceiptPath); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("planning wrote receipt: %v", err)
	}
	if err := ValidateHooks(plan.Receipt.HookDefinition); err != nil {
		t.Fatal(err)
	}
	for _, artifact := range plan.Receipt.Artifacts {
		found := false
		for _, change := range plan.Changes {
			if change.Path == artifact.Path {
				found = Digest(change.After) == artifact.SHA256
			}
		}
		if !found {
			t.Fatalf("artifact not bound to planned bytes: %s", artifact.Path)
		}
	}
	var diff bytes.Buffer
	must(t, WriteDiff(&diff, plan.Changes))
	if !bytes.Contains(diff.Bytes(), []byte("permissions")) {
		t.Fatal("complete settings diff missing")
	}
}

func TestProjectPlanAndInvalidInputs(t *testing.T) {
	t.Parallel()
	options := testOptions(t)
	options.Scope = "project"
	plan, err := PlanClaude(options)
	must(t, err)
	if plan.Receipt.SettingsPath != filepath.Join(options.Root, ".claude", "settings.local.json") {
		t.Fatal("project settings escaped the repository")
	}
	_, identity, err := ProjectIdentity(options.Root)
	must(t, err)
	want := filepath.Join(options.StateDir, "claude", "project", identity, "receipt.json")
	if plan.ReceiptPath != want {
		t.Fatalf("receipt path: %s", plan.ReceiptPath)
	}
	options.Scope = "unknown"
	if _, err := PlanClaude(options); !errors.Is(err, ErrPath) {
		t.Fatalf("invalid scope: %v", err)
	}
	options.Scope = "user"
	options.StateDir = "relative"
	if _, err := PlanClaude(options); !errors.Is(err, ErrPath) {
		t.Fatalf("relative state: %v", err)
	}
}

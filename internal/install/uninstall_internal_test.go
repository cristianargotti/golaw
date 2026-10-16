package install

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestUninstallPreservesForeignSettings(t *testing.T) {
	t.Parallel()
	options := testOptions(t)
	original, err := os.ReadFile("testdata/foreign.json")
	must(t, err)
	settings := filepath.Join(options.ConfigDir, "settings.json")
	must(t, os.WriteFile(settings, original, 0o600))
	plan, err := PlanClaude(options)
	must(t, err)
	var output bytes.Buffer
	writer := &enrollmentDouble{output: &output}
	must(t, Apply(t.Context(), plan, &output, writer))
	calls := writer.calls
	removal, err := PlanUninstall(plan.ReceiptPath)
	must(t, err)
	must(t, Apply(t.Context(), removal, &output, nil))
	if writer.calls != calls {
		t.Fatal("uninstall changed enrollment")
	}
	restored, err := os.ReadFile(settings)
	must(t, err)
	assertJSONEqual(t, original, restored)
	for _, artifact := range plan.Receipt.Artifacts {
		if artifact.Path == settings {
			continue
		}
		if _, err := os.Stat(artifact.Path); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("owned file remains: %s", artifact.Path)
		}
	}
	if _, err := PlanUninstall(plan.ReceiptPath); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("uninstall guessed missing ownership: %v", err)
	}
}

func TestUninstallPreservesChangedArtifact(t *testing.T) {
	t.Parallel()
	options := testOptions(t)
	plan, err := PlanClaude(options)
	must(t, err)
	var output bytes.Buffer
	must(t, Apply(t.Context(), plan, &output, &enrollmentDouble{output: &output}))
	path := plan.Receipt.Artifacts[0].Path
	must(t, os.WriteFile(path, []byte("user edit\n"), 0o600))
	if _, removalErr := PlanUninstall(plan.ReceiptPath); !errors.Is(removalErr, ErrChanged) {
		t.Fatalf("changed artifact accepted: %v", removalErr)
	}
	if _, reinstallErr := PlanClaude(options); !errors.Is(reinstallErr, ErrChanged) {
		t.Fatalf("reinstall overwrote changed artifact: %v", reinstallErr)
	}
	data, err := os.ReadFile(path)
	must(t, err)
	if string(data) != "user edit\n" {
		t.Fatal("changed artifact was removed")
	}
}

func TestUninstallRefusesArtifactsOutsideTheBundle(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	bundle, outside := filepath.Join(base, "bundle"), filepath.Join(base, "outside")
	must(t, os.MkdirAll(bundle, 0o700))
	must(t, os.WriteFile(outside, []byte("owned elsewhere\n"), 0o600))
	artifacts := []Artifact{{Path: outside, SHA256: Digest([]byte("owned elsewhere\n"))}}
	for _, receipt := range []Receipt{
		{Schema: 1, ProjectID: "project", Bundle: bundle, Artifacts: artifacts},
		{Schema: 1, ProjectID: "project", Bundle: outside, Artifacts: artifacts},
	} {
		path := filepath.Join(t.TempDir(), "receipt.json")
		data, err := json.Marshal(receipt)
		must(t, err)
		must(t, os.WriteFile(path, data, 0o600))
		if _, err := PlanUninstall(path); !errors.Is(err, ErrPath) {
			t.Errorf("artifact %s of bundle %s planned for removal: %v", outside, receipt.Bundle, err)
		}
	}
	data, err := os.ReadFile(outside)
	if err != nil || string(data) != "owned elsewhere\n" {
		t.Fatalf("a file outside the bundle changed: %q, %v", data, err)
	}
}

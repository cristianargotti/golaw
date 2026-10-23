package doctor

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/install"
)

// editedSettings names the three groups the installer writes, so each case
// edits one entry without asserting the shape of the others.
type editedSettings struct {
	EnabledPlugins map[string]any `json:"enabledPlugins"`
	Marketplaces   map[string]any `json:"extraKnownMarketplaces"`
	Permissions    struct {
		Deny []string `json:"deny"`
	} `json:"permissions"`
	Model string `json:"model,omitempty"`
}

func TestSettingsArtifactIsJudgedByItsOwnedEntries(t *testing.T) {
	t.Parallel()
	input, path := installedSettings(t)
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, code string
		edit       func(*editedSettings)
	}{
		{"a foreign key", "artifact-matches", func(s *editedSettings) { s.Model = "opus" }},
		{"another plugin", "artifact-matches", func(s *editedSettings) { s.EnabledPlugins["other@other"] = false }},
		{"a foreign deny rule first", "artifact-matches", func(s *editedSettings) {
			s.Permissions.Deny = append([]string{"Bash(rm:*)"}, s.Permissions.Deny...)
		}},
		{"the plugin removed", "artifact-drift", func(s *editedSettings) { delete(s.EnabledPlugins, "golaw@golaw") }},
		{"the marketplace moved", "artifact-drift", func(s *editedSettings) { s.Marketplaces["golaw"] = "elsewhere" }},
		{"the deny rule removed", "artifact-drift", func(s *editedSettings) { s.Permissions.Deny = nil }},
	} {
		var settings editedSettings
		if err := json.Unmarshal(original, &settings); err != nil {
			t.Fatal(err)
		}
		tc.edit(&settings)
		writeJSON(t, path, settings)
		if got := artifactCode(assess(Collect(t.Context(), input, nil, nil)), path); got != tc.code {
			t.Errorf("%s: settings artifact is %q, want %q", tc.name, got, tc.code)
		}
	}
}

func artifactCode(report Report, path string) string {
	for _, line := range report.Lines {
		if line.ID == idArtifacts && line.Key == path {
			return line.Code
		}
	}
	return ""
}

func installedSettings(t *testing.T) (Input, string) {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "repository")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	plan, err := install.PlanClaude(install.Options{
		Root: root, ConfigDir: filepath.Join(base, "config"),
		StateDir: filepath.Join(base, "state"), Version: "0.1.0",
		PolicyDigest: "policy", ReadRules: []string{"Read(**/*.key)"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := install.Apply(t.Context(), plan, io.Discard, &enrollmentStore{}); err != nil {
		t.Fatal(err)
	}
	return Input{Root: root, ReceiptPath: plan.ReceiptPath}, plan.Receipt.SettingsPath
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestAMissingReceiptIsReportedAsEmpty(t *testing.T) {
	t.Parallel()
	input := Input{Root: t.TempDir(), ReceiptPath: filepath.Join(t.TempDir(), "absent.json")}
	if got := artifactCode(assess(Collect(t.Context(), input, nil, nil)), ""); got != "receipt-artifacts-empty" {
		t.Fatalf("a missing receipt was reported as %q", got)
	}
}

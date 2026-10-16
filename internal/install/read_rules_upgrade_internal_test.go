package install

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/paths"
)

func TestInstallReplacesLegacyReadRules(t *testing.T) {
	t.Parallel()
	options, previous, original := installLegacyReadRules(t)
	current, err := ReadRules(paths.SecretGlobs())
	must(t, err)
	options.ReadRules = current
	options.Version = "0.2.0"
	plan, err := PlanClaude(options)
	must(t, err)
	var output bytes.Buffer
	must(t, Apply(t.Context(), plan, &output, &enrollmentDouble{output: &output}))
	installed, err := os.ReadFile(plan.Receipt.SettingsPath)
	must(t, err)
	deny := settingsDenyRules(t, installed)
	for _, rule := range current {
		if !slices.Contains(deny, rule) {
			t.Errorf("current rule missing after upgrade: %s", rule)
		}
	}
	for _, rule := range []string{
		"Read(**/*.tfvars)", "Read(**/*.tfvars.json)",
		"Read(**/*service-account*.json)", "Read(**/*service_account*.json)",
	} {
		if slices.Contains(deny, rule) {
			t.Errorf("legacy rule remains after upgrade: %s", rule)
		}
	}
	foreign := settingsDenyRules(t, original)
	if len(deny) < len(foreign) || !slices.Equal(deny[:len(foreign)], foreign) {
		t.Fatalf("foreign rules changed order: %q", deny)
	}
	removal, err := PlanUninstall(previous.ReceiptPath)
	must(t, err)
	must(t, Apply(t.Context(), removal, &output, nil))
	restored, err := os.ReadFile(plan.Receipt.SettingsPath)
	must(t, err)
	assertJSONEqual(t, original, restored)
}

func TestUninstallRemovesLegacyReadRules(t *testing.T) {
	t.Parallel()
	_, plan, original := installLegacyReadRules(t)
	removal, err := PlanUninstall(plan.ReceiptPath)
	must(t, err)
	var output bytes.Buffer
	must(t, Apply(t.Context(), removal, &output, nil))
	restored, err := os.ReadFile(plan.Receipt.SettingsPath)
	must(t, err)
	assertJSONEqual(t, original, restored)
}

func installLegacyReadRules(t *testing.T) (Options, Plan, []byte) {
	t.Helper()
	options := testOptions(t)
	original, err := os.ReadFile("testdata/foreign.json")
	must(t, err)
	settings := filepath.Join(options.ConfigDir, "settings.json")
	must(t, os.WriteFile(settings, original, 0o600))
	options.ReadRules, err = ReadRules([]string{
		"**/*.pem", "**/*.key", "**/*.p12", "**/*.pfx", "**/*.jks", "**/*.kdbx",
		"**/*.tfvars", "**/*.tfvars.json",
		"~/.ssh/id_rsa", "~/.ssh/id_dsa", "~/.ssh/id_ecdsa", "~/.ssh/id_ed25519",
		"~/.aws/credentials", "~/.config/gcloud/*.json", "~/.azure/**",
		"**/*service-account*.json", "**/*service_account*.json",
	})
	must(t, err)
	plan, err := PlanClaude(options)
	must(t, err)
	var output bytes.Buffer
	must(t, Apply(t.Context(), plan, &output, &enrollmentDouble{output: &output}))
	return options, plan, original
}

func settingsDenyRules(t *testing.T, data []byte) []string {
	t.Helper()
	root, err := settingsObject(data)
	must(t, err)
	deny, _, err := denyValues(root)
	must(t, err)
	return deny
}

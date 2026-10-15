package install

import "testing"

func TestRestoreSettingsRefusesMalformedGroups(t *testing.T) {
	t.Parallel()
	owned := Ownership{Containers: []string{"enabledPlugins"}}
	for _, data := range []string{"{", `{"permissions":5}`, `{"permissions":{"deny":"text"}}`, `{"enabledPlugins":[]}`} {
		if _, err := RestoreSettings([]byte(data), owned); err == nil {
			t.Fatalf("malformed settings accepted: %s", data)
		}
	}
	created := Ownership{Created: true, Containers: []string{"enabledPlugins", "extraKnownMarketplaces", "permissions"}}
	restored, err := RestoreSettings([]byte(`{}`), created)
	if err != nil || restored != nil {
		t.Fatalf("a settings file the installer created was kept: %q, %v", restored, err)
	}
}

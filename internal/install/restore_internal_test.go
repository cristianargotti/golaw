package install

import (
	"bytes"
	"errors"
	"os"
	"testing"
)

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

func TestVerifySettingsJudgesOwnedEntriesOnly(t *testing.T) {
	t.Parallel()
	original, err := os.ReadFile("testdata/foreign.json")
	must(t, err)
	data, owned, err := MergeSettings(original, "/catalog", []string{"Read(**/*.key)"}, Ownership{})
	must(t, err)
	foreign := bytes.Replace(data, []byte(`"model": "fixed"`), []byte(`"model": "opus"`), 1)
	if bytes.Equal(foreign, data) {
		t.Fatal("fixture did not change the foreign key")
	}
	must(t, VerifySettings(data, owned))
	must(t, VerifySettings(foreign, owned))
	for _, edit := range []struct{ old, new string }{
		{`"golaw@golaw": true`, `"golaw@golaw": false`},
		{`"path": "/catalog"`, `"path": "/elsewhere"`},
		{`"Read(**/*.key)"`, `"Read(**/*.crt)"`},
	} {
		changed := bytes.Replace(data, []byte(edit.old), []byte(edit.new), 1)
		if bytes.Equal(changed, data) {
			t.Fatalf("fixture did not change %s", edit.old)
		}
		if err := VerifySettings(changed, owned); !errors.Is(err, ErrChanged) {
			t.Errorf("owned entry changed but verified: %s: %v", edit.new, err)
		}
	}
	for _, malformed := range []string{"{", `{"enabledPlugins":[]}`, `{"permissions":{"deny":7}}`} {
		if err := VerifySettings([]byte(malformed), owned); err == nil {
			t.Fatalf("malformed settings verified: %s", malformed)
		}
	}
}

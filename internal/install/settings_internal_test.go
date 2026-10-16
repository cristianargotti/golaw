package install

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestSettingsOwnershipLifecycle(t *testing.T) {
	t.Parallel()
	original, err := os.ReadFile("testdata/foreign.json")
	if err != nil {
		t.Fatal(err)
	}
	rules := []string{"Read(**/*.pem)", "Read(**/*.key)"}
	first, owned, err := MergeSettings(original, "/catalog", rules, Ownership{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(owned.Deny, []string{"Read(**/*.key)"}) {
		t.Fatalf("preexisting deny became owned: %#v", owned)
	}
	second, next, err := MergeSettings(first, "/catalog", rules, owned)
	if err != nil || !bytes.Equal(first, second) {
		t.Fatalf("second merge changed bytes: %v", err)
	}
	serialized, err := json.Marshal(next)
	if err != nil {
		t.Fatal(err)
	}
	var loaded Ownership
	if decodeErr := json.Unmarshal(serialized, &loaded); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	restored, err := RestoreSettings(second, loaded)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, original, restored)
	updated, upgrade, err := MergeSettings(first, "/new", []string{"Read(**/*.tfvars)"}, owned)
	if err != nil {
		t.Fatal(err)
	}
	restored, err = RestoreSettings(updated, upgrade)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, original, restored)
}

func TestSettingsEmptyMalformedAndChanged(t *testing.T) {
	t.Parallel()
	for _, data := range []string{"null", "[]", "{", `{"permissions":{"deny":7}}`} {
		if _, _, err := MergeSettings([]byte(data), "/catalog", nil, Ownership{}); err == nil {
			t.Fatalf("malformed settings accepted: %s", data)
		}
	}
	data, owned, err := MergeSettings(nil, "/catalog", nil, Ownership{})
	if err != nil {
		t.Fatal(err)
	}
	restored, err := RestoreSettings(data, owned)
	if err != nil || restored != nil {
		t.Fatalf("created settings not removed: %s %v", restored, err)
	}
	data, owned, err = MergeSettings([]byte(`{"enabledPlugins":{"golaw@golaw":null}}`),
		"/catalog", nil, Ownership{})
	if err != nil {
		t.Fatal(err)
	}
	restored, err = RestoreSettings(data, owned)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, []byte(`{"enabledPlugins":{"golaw@golaw":null}}`), restored)
	changed := bytes.Replace(data, []byte(`"golaw@golaw": true`), []byte(`"golaw@golaw": false`), 1)
	restored, err = RestoreSettings(changed, owned)
	if err != nil || !bytes.Contains(restored, []byte(`"golaw@golaw": false`)) {
		t.Fatalf("user edit removed: %s %v", restored, err)
	}
}

func TestReadRuleFamilies(t *testing.T) {
	t.Parallel()
	globs := []string{
		"**/.env", "**/.env.*", "**/.envrc", "**/secrets/**",
		"**/credentials/**/*.json", "**/keys/**/*.json",
		"**/*kubeconfig*", "**/.kube/**", "**/.docker/config.json",
		"**/.vault-token", "**/*.tfstate", "**/*.tfstate.*",
		"**/*.tfvars", "**/*.tfvars.json", "**/*.pem", "**/*.key",
		"**/*.p12", "**/*.pfx", "**/.ssh/id_*",
		"**/.aws/credentials", "**/application_default_credentials.json",
		"**/.netrc", "**/.bashrc", "**/.zshrc", "**/.profile",
	}
	rules, err := ReadRules(globs)
	if err != nil || len(rules) != len(globs) {
		t.Fatalf("rules: %#v %v", rules, err)
	}
	for index, glob := range globs {
		if rules[index] != "Read("+glob+")" {
			t.Fatalf("rule widened: %s", rules[index])
		}
	}
	for _, glob := range []string{"", "**/*.key)\nBash(*)", "credentials.go"} {
		if _, err := ReadRules([]string{glob}); err == nil {
			t.Fatalf("invalid secret glob accepted: %q", glob)
		}
	}
}

func assertJSONEqual(t *testing.T, left, right []byte) {
	t.Helper()
	var a, b any
	if err := json.Unmarshal(left, &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(right, &b); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("JSON differs:\n%s\n%s", left, right)
	}
}

func TestMergeKeepsForeignBytesAndKeyOrder(t *testing.T) {
	t.Parallel()
	original := []byte(`{"model":"fixed","permissions":{"defaultMode":"plan","deny":["Bash(echo <done>)"],` +
		`"allow":["Bash(make && make test)"]},"hooks":{"PreToolUse":[{"matcher":"Bash",` +
		`"hooks":[{"type":"command","command":"test -f a && echo <ok>"}]}]}}`)
	merged, owned, err := MergeSettings(original, "/catalog", []string{"Read(private/**)"}, Ownership{})
	must(t, err)
	for _, fragment := range []string{`"test -f a && echo <ok>"`, `"Bash(echo <done>)"`, `"Bash(make && make test)"`} {
		if !bytes.Contains(merged, []byte(fragment)) {
			t.Errorf("foreign bytes changed: %s\n%s", fragment, merged)
		}
	}
	position := 0
	for _, key := range []string{
		`"model"`, `"permissions"`, `"defaultMode"`, `"deny"`, `"allow"`, `"hooks"`,
		`"enabledPlugins"`, `"extraKnownMarketplaces"`,
	} {
		offset := bytes.Index(merged[position:], []byte(key))
		if offset < 0 {
			t.Fatalf("key %s missing or out of order:\n%s", key, merged)
		}
		position += offset + len(key)
	}
	restored, err := RestoreSettings(merged, owned)
	must(t, err)
	var compact bytes.Buffer
	must(t, json.Compact(&compact, restored))
	if !bytes.Equal(compact.Bytes(), original) {
		t.Fatalf("uninstall did not restore the original document:\n%s", restored)
	}
}

package policyfiles_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestSquawkConfig(t *testing.T) {
	t.Parallel()
	const id = "policy.squawk-config"
	const path = ".squawk.toml"
	const code = "squawk-policy"
	red := depsProof(t, id, "red", "go", "db")
	near := depsProof(t, id, "near", "go", "db")
	body := near.files[path]
	assertDeps(t, policyfiles.SquawkConfig(), []depsCase{
		{name: "transaction red", input: red, want: []string{code}},
		{name: "configured near", input: near},
		{
			name:  "no exclusions is stronger",
			input: depsEdit(near, path, strings.ReplaceAll(body, `["prefer-identity"]`, "[]")),
		},
		{
			name:  "different version near",
			input: depsEdit(near, path, strings.ReplaceAll(body, `"17.0"`, `"16.4"`)),
		},
		{
			name: "extra excluded rule red",
			input: depsEdit(near, path,
				strings.ReplaceAll(body, `["prefer-identity"]`,
					`["prefer-identity", "require-concurrent-index-creation"]`)),
			want: []string{code},
		},
		{
			name:  "version missing red",
			input: depsEdit(near, path, "assume_in_transaction = true\n"),
			want:  []string{code},
		},
		{name: "file missing red", input: depsEdit(near, path, ""), want: []string{code}},
		{
			name: "excluded migration red",
			input: depsEdit(depsEdit(near, path, body+"excluded_paths = [\"db/migrations/0001_legacy.sql\"]\n"),
				"db/migrations/0001_legacy.sql", "CREATE TABLE legacy (id bigint);\n"),
			want: []string{code},
		},
		{
			name:  "excluded migrations directory red",
			input: depsEdit(near, path, body+"excluded_paths = [\"db/migrations/*\"]\n"),
			want:  []string{code},
		},
		{
			name:  "excluded unrelated path near",
			input: depsEdit(near, path, body+"excluded_paths = [\"vendor/*\"]\n"),
		},
		{
			name:  "valid unrelated table",
			input: depsEdit(near, path, body+"[upload_to_github]\nfail_on_violations = true\n"),
		},
		{
			name: "malformed TOML", input: depsEdit(near, path, "pg_version = [\n"),
			status: model.StatusUnmeasured, reason: path,
		},
		{
			name: "capability off", input: depsProof(t, id, "red", "go"),
			status: model.StatusNotApplicable, reason: "capability db",
		},
	})
}

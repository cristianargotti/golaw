package policyfiles_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestRedoclyIgnoreLocations(t *testing.T) {
	t.Parallel()
	red := depsProof(t, "policy.openapi-assertions", "red", "api")
	for _, row := range []struct {
		name, body string
		red        bool
	}{
		{"schema pointer", "api/openapi.yaml:\n  rule/number-double: ['#/components/schemas/Amount']\n", false},
		{"normalized path", "./api/openapi.yaml:\n  rule/number-double: ['#/components/schemas/Amount/format']\n", false},
		{"another file", "api/other.yaml:\n  rule/number-double: ['#/components/schemas/Amount/format']\n", true},
		{"another rule", "api/openapi.yaml:\n  rule/response-example: ['#/components/schemas/Amount/format']\n", true},
		{"another node", "api/openapi.yaml:\n  rule/number-double: ['#/components/schemas/Other']\n", true},
		{"all suppressed and invalid unrelated", "api/openapi.yaml:\n  rule/number-double: ['#/components/schemas/Amount']\nother.yaml: false\n", false},
		{"empty ignore", "# empty\n", true},
	} {
		var want []string
		if row.red {
			want = []string{"openapi-policy"}
		}
		assertDeps(t, policyfiles.OpenAPIAssertions(), []depsCase{{
			name: row.name, input: depsEdit(red, ".redocly.lint-ignore.yaml", row.body), want: want,
		}})
	}
	renamed := depsEdit(red, "redocly.yaml", strings.ReplaceAll(red.files["redocly.yaml"], "rule/number-double", "rule/precise"))
	assertDeps(t, policyfiles.OpenAPIAssertions(), []depsCase{{
		name: "configured custom rule name", input: depsEdit(renamed, ".redocly.lint-ignore.yaml",
			"api/openapi.yaml:\n  rule/precise: ['#/components/schemas/Amount/format']\n"),
	}})
}

func TestRedoclyIgnoreLimitsOnlyWithFindings(t *testing.T) {
	t.Parallel()
	red := depsProof(t, "policy.openapi-assertions", "red", "api")
	near := depsProof(t, "policy.openapi-assertions", "near", "api")
	for _, row := range []struct{ name, body string }{
		{"contract scalar", "api/openapi.yaml: true\n"},
		{"contract list", "api/openapi.yaml: [everything]\n"},
		{"rule scalar", "api/openapi.yaml: {rule/number-double: false}\n"},
		{"rule mapping", "api/openapi.yaml: {rule/number-double: {all: true}}\n"},
		{"rule number", "api/openapi.yaml: {rule/number-double: [1]}\n"},
		{"rule boolean", "api/openapi.yaml: {rule/number-double: [true]}\n"},
		{"escaping path", "../api/openapi.yaml: {rule/number-double: []}\n"},
		{"absolute path", "/api/openapi.yaml: {rule/number-double: []}\n"},
		{"malformed root", "[everything]\n"},
	} {
		assertDeps(t, policyfiles.OpenAPIAssertions(), []depsCase{
			{name: row.name + " with finding", input: depsEdit(red, ".redocly.lint-ignore.yaml", row.body), status: model.StatusUnmeasured, reason: ".redocly.lint-ignore.yaml", outside: true},
			{name: row.name + " without finding", input: depsEdit(near, ".redocly.lint-ignore.yaml", row.body)},
		})
	}
	assertDeps(t, policyfiles.OpenAPIAssertions(), []depsCase{
		{name: "syntax with finding", input: depsEdit(red, ".redocly.lint-ignore.yaml", "a: [\n"), status: model.StatusUnmeasured, reason: ".redocly.lint-ignore.yaml"},
		{name: "syntax without finding", input: depsEdit(near, ".redocly.lint-ignore.yaml", "a: [\n")},
	})
}

package policyfiles_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestRedoclyEffectiveAssertions(t *testing.T) {
	t.Parallel()
	near := depsProof(t, "policy.openapi-assertions", "near", "api")
	config := near.files["redocly.yaml"]
	for _, row := range []struct {
		name, old, next string
		red             bool
	}{
		{"custom number name", "rule/number-double:", "rule/precise:", false},
		{"custom response name", "rule/response-example:", "rule/illustrated:", false},
		{"explicit error", "subject: {type: Schema, property: format}", "severity: error\n    subject: {type: Schema, property: format}", false},
		{"warning assertion", "subject: {type: Schema, property: format}", "severity: warn\n    subject: {type: Schema, property: format}", true},
		{"disabled assertion", "subject: {type: Schema, property: format}", "severity: off\n    subject: {type: Schema, property: format}", true},
		{"weak format", "const: double", "const: float", true},
		{"undefined format", "defined: true, const: double", "defined: false, const: double", true},
		{"missing defined", "defined: true, const: double", "const: double", true},
		{"weak response", "requireAny: [example, examples]", "requireAny: [example, examples, schema]", true},
		{"stricter response", "requireAny: [example, examples]", "requireAny: [example]", false},
		{"severity mapping", "no-invalid-media-type-examples: error", "no-invalid-media-type-examples: {severity: error}", false},
		{"warning validation", "no-invalid-media-type-examples: error", "no-invalid-media-type-examples: warn", true},
		{"per API disable", "root: api/openapi.yaml", "root: api/openapi.yaml\n    rules:\n      rule/number-double: off", true},
		{"per API severity", "root: api/openapi.yaml", "root: api/openapi.yaml\n    rules:\n      rule/number-double: {severity: error}", false},
		{"per API assertion override", "root: api/openapi.yaml", "root: api/openapi.yaml\n    rules:\n      rule/number-double: {assertions: {const: double}}", false},
		{"per API subject override", "root: api/openapi.yaml", "root: api/openapi.yaml\n    rules:\n      rule/number-double: {subject: {property: format}}", false},
		{"per API validator", "root: api/openapi.yaml", "root: api/openapi.yaml\n    rules:\n      no-invalid-media-type-examples: off", true},
		{"per API unrelated", "root: api/openapi.yaml", "root: api/openapi.yaml\n    rules:\n      unrelated: off", false},
		{"unrelated rule", "rules:\n", "rules:\n  rule/unrelated: {subject: {type: Info}, assertions: {defined: true}}\n", false},
		{"unrelated config", "extends: [recommended]", "extends: [recommended]\nother: {arbitrary: [true]}", false},
		{"empty plugins", "extends: [recommended]", "extends: [recommended]\nplugins: []", false},
	} {
		var want []string
		if row.red {
			want = []string{"openapi-policy"}
		}
		assertDeps(t, policyfiles.OpenAPIAssertions(), []depsCase{{
			name: row.name, input: depsEdit(near, "redocly.yaml", strings.Replace(config, row.old, row.next, 1)), want: want,
		}})
	}
}

func TestRedoclyConfigurationLimits(t *testing.T) {
	t.Parallel()
	near := depsProof(t, "policy.openapi-assertions", "near", "api")
	config := near.files["redocly.yaml"]
	for _, row := range []struct{ name, old, next, reason string }{
		{"apis type", "apis:", "apis: []\nunused:", "apis"},
		{"scalar plugins", "extends: [recommended]", "extends: [recommended]\nplugins: ./plugin.js", "plugins"},
		{"validation severity type", "no-invalid-media-type-examples: error", "no-invalid-media-type-examples: true", "severity"},
		{"custom severity type", "subject: {type: Schema, property: format}", "severity: 1\n    subject: {type: Schema, property: format}", "severity"},
		{"string boolean assertion", "defined: true, const: double", "defined: 'true', const: double", "assertions"},
		{"response property scope", "subject: {type: MediaType}", "subject: {type: MediaType, property: schema}", "subject"},
		{"plugins", "extends: [recommended]", "extends: [recommended]\nplugins: [./plugin.js]", "plugins"},
		{"extends type", "[recommended]", "{recommended: true}", "extends"},
		{"mixed extends", "[recommended]", "[recommended, true]", "extends"},
		{"remote root", "root: api/openapi.yaml", "root: https://example.test/api.yaml", "root"},
		{"escaping root", "root: api/openapi.yaml", "root: ../api.yaml", "root"},
		{"computed root", "root: api/openapi.yaml", "root: $API", "root"},
		{"rules type", "rules:\n", "rules: []\nunused:\n", "rules"},
		{"custom assertion", "const: double", "pattern: double", "assertions"},
		{"custom filter", "const: number", "pattern: number", "where"},
		{"extended filter", "subject: {type: Response}", "subject: {type: Response, property: content}", "where"},
		{"complex subject", "type: Schema, property: format", "type: Schema, property: format, filter: special", "expression"},
		{"response assertion type", "requireAny: [example, examples]", "requireAny: [example, true]", "assertions"},
	} {
		assertDeps(t, policyfiles.OpenAPIAssertions(), []depsCase{{
			name: row.name, input: depsEdit(near, "redocly.yaml", strings.Replace(config, row.old, row.next, 1)),
			status: model.StatusUnmeasured, reason: row.reason, outside: true,
		}})
	}
}

func TestRedoclyRequiredConfiguration(t *testing.T) {
	t.Parallel()
	near := depsProof(t, "policy.openapi-assertions", "near", "api")
	assertDeps(t, policyfiles.OpenAPIAssertions(), []depsCase{
		{name: "empty configuration", input: depsEdit(near, "redocly.yaml", "# empty\n"), want: []string{"openapi-policy"}},
		{name: "missing rules", input: depsEdit(near, "redocly.yaml", "apis: {main: {root: api/openapi.yaml}}\n"), want: []string{"openapi-policy"}},
		{name: "configuration syntax", input: depsEdit(near, "redocly.yaml", "apis: [\n"), status: model.StatusUnmeasured, reason: "redocly.yaml"},
	})
}

func TestRedoclyAliasesPreserveEffectiveRules(t *testing.T) {
	t.Parallel()
	near := depsProof(t, "policy.openapi-assertions", "near", "api")
	config := near.files["redocly.yaml"]
	config = strings.Replace(config, "rule/number-double:", "rule/number-double: &number", 1)
	config += "  rule/number-copy: *number\n"
	local := strings.Replace(config, "apis:\n  main:\n    root: api/openapi.yaml\n", "", 1)
	assertDeps(t, policyfiles.OpenAPIAssertions(), []depsCase{
		{name: "global rule alias", input: depsEdit(near, "redocly.yaml", config)},
		{name: "API rule alias", input: depsEdit(near, "redocly.yaml", local+"apis:\n  main:\n    root: api/openapi.yaml\n    rules: {rule/number-double: *number}\n")},
	})
}

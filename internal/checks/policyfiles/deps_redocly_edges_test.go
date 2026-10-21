package policyfiles_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestRedoclyUnrelatedConfigurableRules(t *testing.T) {
	t.Parallel()
	near := depsProof(t, "policy.openapi-assertions", "near", "api")
	for _, row := range []struct{ name, rule string }{
		{"schema description", "{subject: {type: Schema}, assertions: {required: [description]}}"},
		{"string format", "{subject: {type: Schema, property: format}, where: [{subject: {type: Schema, property: type}, assertions: {const: string}}], assertions: {enum: [date, date-time]}}"},
		{"media schema", "{subject: {type: MediaType, property: schema}, assertions: {defined: true}}"},
	} {
		config := strings.Replace(near.files["redocly.yaml"], "rules:\n", "rules:\n  rule/unrelated: "+row.rule+"\n", 1)
		assertDeps(t, policyfiles.OpenAPIAssertions(), []depsCase{{
			name: row.name, input: depsEdit(near, "redocly.yaml", config),
		}})
	}
}

func TestRedoclyUnrecognizedRuleNeedsRelevantAssertion(t *testing.T) {
	t.Parallel()
	near := depsProof(t, "policy.openapi-assertions", "near", "api")
	for _, row := range []struct{ name, old, next string }{
		{"number rule", "subject: {type: Schema, property: format}", "subject: {type: Schema}"},
		{"response rule", "subject: {type: MediaType}", "subject: {type: MediaType, property: schema}"},
	} {
		config := strings.Replace(near.files["redocly.yaml"], row.old, row.next, 1)
		assertDeps(t, policyfiles.OpenAPIAssertions(), []depsCase{{
			name: row.name, input: depsEdit(near, "redocly.yaml", config), status: model.StatusUnmeasured,
			reason: "rule/", outside: true,
		}})
	}
}

func TestRedoclyConfigurableRuleAnnotations(t *testing.T) {
	t.Parallel()
	near := depsProof(t, "policy.openapi-assertions", "near", "api")
	for _, key := range []string{"message", "suggest", "documentationLink"} {
		config := strings.Replace(near.files["redocly.yaml"], "rule/number-double:\n",
			"rule/number-double:\n    "+key+": 'details for the rule'\n", 1)
		assertDeps(t, policyfiles.OpenAPIAssertions(), []depsCase{{
			name: key, input: depsEdit(near, "redocly.yaml", config),
		}})
	}
}

func TestRedoclyGlobalRulesDefaultContract(t *testing.T) {
	t.Parallel()
	near := depsProof(t, "policy.openapi-assertions", "near", "api")
	config := strings.Replace(near.files["redocly.yaml"], "apis:\n  main:\n    root: api/openapi.yaml\n", "", 1)
	red := strings.Replace(near.files["api/openapi.yaml"], "format: double", "format: float", 1)
	assertDeps(t, policyfiles.OpenAPIAssertions(), []depsCase{
		{name: "no apis", input: depsEdit(near, "redocly.yaml", config)},
		{name: "empty apis", input: depsEdit(near, "redocly.yaml", config+"apis: {}\n")},
		{name: "missing source", input: depsEdit(depsEdit(near, "redocly.yaml", config), "api/openapi.yaml", "")},
		{name: "default contract inspected", input: depsEdit(depsEdit(near, "redocly.yaml", config), "api/openapi.yaml", red), want: []string{"openapi-policy"}},
	})
	got := measure(t, policyfiles.OpenAPIAssertions(), depsEdit(depsEdit(near, "redocly.yaml", config), "api/openapi.yaml", red))
	if len(got.Findings) != 1 || got.Findings[0].Path != "api/openapi.yaml" {
		t.Fatalf("findings = %+v, want the default contract violation", got.Findings)
	}
}

func TestRedoclyRecognizedWeakRuleRemainsFinding(t *testing.T) {
	t.Parallel()
	near := depsProof(t, "policy.openapi-assertions", "near", "api")
	config := strings.Replace(near.files["redocly.yaml"], "rule/number-double:", "rule/number-double: &number", 1)
	config += "  rule/weak-number:\n    <<: *number\n    severity: warn\n"
	assertDeps(t, policyfiles.OpenAPIAssertions(), []depsCase{{
		name: "valid duplicate does not hide weak assertion", input: depsEdit(near, "redocly.yaml", config), want: []string{"openapi-policy"},
	}})
}

func TestRedoclyRuleRepairMessages(t *testing.T) {
	t.Parallel()
	near := depsProof(t, "policy.openapi-assertions", "near", "api")
	for _, row := range []struct{ name, old, next, repair string }{
		{"validation", "no-invalid-media-type-examples: error", "no-invalid-media-type-examples: off", "no-invalid-media-type-examples: error"},
		{"number assertion", "const: double", "const: float", "rule/number-double: {severity: error, assertions: {defined: true, const: double}}"},
		{"number severity", "subject: {type: Schema, property: format}", "severity: warn\n    subject: {type: Schema, property: format}", "rule/number-double: {severity: error, assertions: {defined: true, const: double}}"},
		{"response", "requireAny: [example, examples]", "requireAny: [example, examples, schema]", "rule/response-example: {severity: error, assertions: {requireAny: [example, examples]}}"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			config := strings.Replace(near.files["redocly.yaml"], row.old, row.next, 1)
			got := measure(t, policyfiles.OpenAPIAssertions(), depsEdit(near, "redocly.yaml", config))
			if len(got.Findings) != 1 || !strings.Contains(got.Findings[0].Message, row.repair) {
				t.Fatalf("findings = %+v, want repair %q", got.Findings, row.repair)
			}
		})
	}
}

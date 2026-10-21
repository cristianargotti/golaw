package policyfiles_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestOpenAPIAssertions(t *testing.T) {
	t.Parallel()
	const id = "policy.openapi-assertions"
	const document = "api/openapi.yaml"
	const config = "redocly.yaml"
	const ignore = ".redocly.lint-ignore.yaml"
	const code = "openapi-policy"
	const response = "openapi: 3.0.3\ninfo: {title: sample, version: 1.0.0}\n" +
		"paths:\n  /amount:\n    get:\n      responses:\n        '200':\n" +
		"          description: amount\n          content:\n            application/json:\n" +
		"              schema: {type: number, format: double}\n"
	red := depsProof(t, id, "red", "go", "api")
	near := depsProof(t, id, "near", "go", "api")
	assertDeps(t, policyfiles.OpenAPIAssertions(), []depsCase{
		{name: "float red", input: red, want: []string{code}},
		{name: "double near", input: near},
		{
			name: "integer near",
			input: depsEdit(near, document,
				strings.ReplaceAll(near.files[document], "type: number", "type: integer")),
		},
		{name: "response lacks example", input: depsEdit(near, document, response), want: []string{code}},
		{name: "singular example", input: depsEdit(near, document, response+"              example: 1.5\n")},
		{
			name: "named example",
			input: depsEdit(near, document,
				response+"              examples:\n                amount: {value: 1.5}\n"),
		},
		{
			name: "bodyless response near",
			input: depsEdit(near, document,
				"openapi: 3.0.3\ninfo: {title: sample, version: 1.0.0}\n"+
					"paths:\n  /amount:\n    delete:\n      responses:\n"+
					"        '204': {description: removed}\n"),
		},
		{
			name: "disabled validation red",
			input: depsEdit(near, config, strings.ReplaceAll(near.files[config],
				"no-invalid-media-type-examples: error", "no-invalid-media-type-examples: off")),
			want: []string{code},
		},
		{
			name: "missing configuration", input: depsEdit(near, config, ""),
			want: []string{code},
		},
		{name: "new asset before contract exists", input: depsEdit(near, document, "")},
		{
			name: "example payload is not a schema",
			input: depsEdit(near, document,
				response+"              example: {type: number, format: float}\n"),
		},
		{
			name: "remote reference",
			input: depsEdit(near, document,
				"openapi: 3.0.3\npaths: {}\ncomponents:\n  schemas:\n"+
					"    Amount: {$ref: 'https://example.test/amount.yaml'}\n"),
			status: model.StatusUnmeasured, reason: "remote reference", outside: true,
		},
		{
			name:   "unsupported document version",
			input:  depsEdit(near, document, "swagger: '2.0'\npaths: {}\n"),
			status: model.StatusUnmeasured, reason: "Swagger 2.0", outside: true,
		},
		{
			name: "custom ruleset limit",
			input: depsEdit(near, config,
				strings.ReplaceAll(near.files[config], "[recommended]", "[./custom-rules.yaml]")),
			status: model.StatusUnmeasured, reason: "custom-rules.yaml", outside: true,
		},
		{
			name: "stricter built-in ruleset near",
			input: depsEdit(near, config,
				strings.ReplaceAll(near.files[config], "[recommended]", "[recommended-strict]")),
		},
		{
			name: "ignored where Redocly ignores it near",
			input: depsEdit(red, ignore,
				"api/openapi.yaml:\n  rule/number-double:\n    - '#/components/schemas/Amount/format'\n"),
		},
		{
			name: "ignore of another location red",
			input: depsEdit(red, ignore,
				"api/openapi.yaml:\n  rule/number-double:\n    - '#/components/schemas/Other/format'\n"),
			want: []string{code},
		},
		{
			name:   "ignore file outside its subset",
			input:  depsEdit(red, ignore, "api/openapi.yaml: [everything]\n"),
			status: model.StatusUnmeasured, reason: ignore, outside: true,
		},
		{
			name: "syntax error", input: depsEdit(near, document, "openapi: [\n"),
			status: model.StatusUnmeasured, reason: document,
		},
		{
			name: "capability off", input: depsProof(t, id, "red", "go"),
			status: model.StatusNotApplicable, reason: "capability api",
		},
	})
}

func TestOpenAPILocalReferences(t *testing.T) {
	t.Parallel()
	const document = "api/openapi.yaml"
	base := depsProof(t, "policy.openapi-assertions", "near", "go", "api")
	base = depsEdit(base, document,
		"openapi: 3.0.3\npaths: {}\ncomponents:\n  schemas:\n"+
			"    Amount: {$ref: './schemas.yaml#/Amount'}\n")
	near := depsEdit(base, "api/schemas.yaml",
		"Amount: {type: number, format: double}\n")
	red := depsEdit(base, "api/schemas.yaml",
		"Amount: {type: number, format: float}\n")
	assertDeps(t, policyfiles.OpenAPIAssertions(), []depsCase{
		{name: "local reference red", input: red, want: []string{"openapi-policy"}},
		{name: "local reference near", input: near},
		{
			name: "missing referenced file", input: base,
			status: model.StatusUnmeasured, reason: "api/schemas.yaml",
		},
		{
			name: "recursive schema terminates",
			input: depsEdit(base, "api/schemas.yaml",
				"Amount:\n  type: object\n  properties:\n"+
					"    next: {$ref: '#/Amount'}\n"),
		},
	})
}

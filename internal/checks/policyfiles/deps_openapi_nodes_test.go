package policyfiles_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestOpenAPISchemaLocations(t *testing.T) {
	t.Parallel()
	base := depsProof(t, "policy.openapi-assertions", "near", "api")
	for _, row := range []struct {
		name, schema string
		red          bool
	}{
		{"properties", "{type: object, properties: {total: {type: number}}}", true},
		{"extension-named property", "{properties: {x-total: {type: number}}}", true},
		{"items", "{type: array, items: {type: number, format: float}}", true},
		{"additional properties", "{additionalProperties: {type: number}}", true},
		{"boolean additional properties", "{additionalProperties: false}", false},
		{"allOf", "{allOf: [{type: number}]}", true},
		{"anyOf", "{anyOf: [{type: number}]}", true},
		{"oneOf", "{oneOf: [{type: number}]}", true},
		{"not", "{not: {type: number}}", true},
		{"opaque schema values", "{type: object, default: {type: number}, example: {type: number}, x-note: {type: number}}", false},
		{"recursive alias", "&loop {type: object, properties: {next: *loop, total: {type: number}}}", true},
	} {
		var want []string
		if row.red {
			want = []string{"openapi-policy"}
		}
		body := "openapi: 3.0.3\ncomponents:\n  schemas:\n    Thing: " + row.schema + "\n"
		assertDeps(t, policyfiles.OpenAPIAssertions(), []depsCase{{
			name: row.name, input: depsEdit(base, "api/openapi.yaml", body), want: want,
		}})
	}
}

func TestOpenAPIOperationLocations(t *testing.T) {
	t.Parallel()
	base := depsProof(t, "policy.openapi-assertions", "near", "api")
	for _, row := range []struct {
		name, body string
		red        bool
	}{
		{"path parameter", "paths: {/value: {parameters: [{schema: {type: number}}]}}", true},
		{"operation parameter", "paths: {/value: {get: {parameters: [{schema: {type: number}}]}}}", true},
		{"parameter content", "components: {parameters: {value: {content: {application/json: {schema: {type: number}}}}}}", true},
		{"request body schema", "components: {requestBodies: {value: {content: {application/json: {schema: {type: number}}}}}}", true},
		{"request needs no example", "paths: {/value: {post: {requestBody: {content: {application/json: {schema: {type: number, format: double}}}}}}}", false},
		{"response header", "components: {responses: {ok: {headers: {total: {schema: {type: number}}}}}}", true},
		{"component header", "components: {headers: {total: {schema: {type: number}}}}", true},
		{"response component", "components: {responses: {ok: {content: {application/json: {schema: {type: string}}}}}}", true},
		{"callback", "paths: {/value: {post: {callbacks: {updated: {'{$request.body#/url}': {post: {responses: {'200': {content: {application/json: {schema: {type: string}}}}}}}}}}}}", true},
		{"callback component", "components: {callbacks: {updated: {'{$request.body#/url}': {post: {responses: {'204': {description: done}}}}}}}", false},
		{"extensions", "paths: {x-note: {type: number}, /value: {x-note: {schema: {type: number}}, get: {responses: {x-note: {type: number}}}}}", false},
	} {
		var want []string
		if row.red {
			want = []string{"openapi-policy"}
		}
		assertDeps(t, policyfiles.OpenAPIAssertions(), []depsCase{{
			name: row.name, input: depsEdit(base, "api/openapi.yaml", "openapi: 3.0.3\n"+row.body+"\n"), want: want,
		}})
	}
}

func TestOpenAPIResponseExamplePresence(t *testing.T) {
	t.Parallel()
	base := depsProof(t, "policy.openapi-assertions", "near", "api")
	for _, row := range []struct {
		name, media string
		red         bool
	}{
		{"no example", "{schema: {type: string}}", true},
		{"empty examples", "{examples: {}}", true},
		{"null example", "{example: null}", false},
		{"empty value example", "{examples: {empty: {value: null}}}", false},
		{"external example left to linter", "{examples: {external: {externalValue: 'https://example.test/example.json'}}}", false},
	} {
		var want []string
		if row.red {
			want = []string{"openapi-policy"}
		}
		body := "openapi: 3.0.3\ncomponents: {responses: {ok: {content: {application/json: " + row.media + "}}}}\n"
		assertDeps(t, policyfiles.OpenAPIAssertions(), []depsCase{{
			name: row.name, input: depsEdit(base, "api/openapi.yaml", body), want: want,
		}})
	}
}

func TestOpenAPIUnsupportedShapes(t *testing.T) {
	t.Parallel()
	base := depsProof(t, "policy.openapi-assertions", "near", "api")
	for _, row := range []struct{ name, body string }{
		{"components", "components: []"},
		{"paths", "paths: []"},
		{"operation", "paths: {/value: {get: []}}"},
		{"parameters", "paths: {/value: {parameters: {name: x}}}"},
		{"schema", "components: {schemas: {Thing: text}}"},
		{"schema type", "components: {schemas: {Thing: {type: {number: true}}}}"},
		{"schema format", "components: {schemas: {Thing: {type: number, format: []}}}"},
		{"examples", "components: {responses: {ok: {content: {application/json: {examples: []}}}}}"},
		{"composition", "components: {schemas: {Thing: {allOf: {type: number}}}}"},
	} {
		assertDeps(t, policyfiles.OpenAPIAssertions(), []depsCase{{
			name: row.name, input: depsEdit(base, "api/openapi.yaml", "openapi: 3.0.3\n"+row.body+"\n"),
			status: model.StatusUnmeasured, reason: "api/openapi.yaml", outside: true,
		}})
	}
}

func TestOpenAPIPathReferenceSiblings(t *testing.T) {
	t.Parallel()
	base := depsProof(t, "policy.openapi-assertions", "near", "api")
	base = depsEdit(base, "api/path.yaml", "delete: {responses: {'204': {description: done}}}\n")
	assertDeps(t, policyfiles.OpenAPIAssertions(), []depsCase{
		{
			name: "local operation beside path reference",
			input: depsEdit(base, "api/openapi.yaml", "openapi: 3.0.3\npaths: {/value: {$ref: './path.yaml', "+
				"get: {responses: {'200': {content: {application/json: {schema: {type: string}}}}}}}}\n"),
			want: []string{"openapi-policy"},
		},
		{
			name: "ambiguous overlapping operations",
			input: depsEdit(base, "api/openapi.yaml", "openapi: 3.0.3\npaths: {/value: {$ref: './path.yaml', "+
				"delete: {responses: {'204': {description: done}}}}}\n"),
			status: model.StatusUnmeasured, reason: "overlapping Path Item", outside: true,
		},
		{
			name: "schema reference ignores siblings in 3.0",
			input: depsEdit(base, "api/openapi.yaml", "openapi: 3.0.3\ncomponents: {schemas: {"+
				"Amount: {type: number, format: double}, Alias: {$ref: '#/components/schemas/Amount', type: number, format: float}}}\n"),
		},
	})
}

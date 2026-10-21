package policyfiles_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
)

func TestOpenAPISchemaDialectForms(t *testing.T) {
	t.Parallel()
	base := depsProof(t, "policy.openapi-assertions", "near", "api")
	for _, version := range []string{"3.0.3", "3.1.0"} {
		for _, row := range []struct{ name, schema string }{
			{"union type", "{type: [number, 'null'], format: float}"},
			{"single type sequence", "{type: [number]}"},
			{"true schema", "true"},
			{"false schema", "false"},
			{"boolean items", "{items: false}"},
			{"boolean properties", "{properties: {x: true, y: false}}"},
			{"boolean composition", "{allOf: [true, false]}"},
		} {
			body := "openapi: " + version + "\ncomponents: {schemas: {Thing: " + row.schema + "}}\n"
			assertDeps(t, policyfiles.OpenAPIAssertions(), []depsCase{{
				name: version + "/" + row.name, input: depsEdit(base, "api/openapi.yaml", body),
			}})
		}
	}
}

func TestOpenAPI31SchemaKeywords(t *testing.T) {
	t.Parallel()
	base := depsProof(t, "policy.openapi-assertions", "near", "api")
	for _, row := range []struct{ keyword, before, after string }{
		{"prefixItems", "[", "]"},
		{"$defs", "{Amount: ", "}"},
		{"dependentSchemas", "{amount: ", "}"},
		{"patternProperties", "{'^x-': ", "}"},
		{"contains", "", ""},
		{"if", "", ""},
		{"then", "", ""},
		{"else", "", ""},
		{"propertyNames", "", ""},
		{"unevaluatedItems", "", ""},
		{"unevaluatedProperties", "", ""},
		{"contentSchema", "", ""},
	} {
		for _, schema := range []string{"{type: number, format: float}", "{type: number, format: double}", "false"} {
			var want []string
			if schema == "{type: number, format: float}" {
				want = []string{"openapi-policy"}
			}
			body := "openapi: 3.1.0\ncomponents: {schemas: {Thing: {" + row.keyword + ": " + row.before + schema + row.after + "}}}\n"
			assertDeps(t, policyfiles.OpenAPIAssertions(), []depsCase{{
				name: row.keyword + "/" + schema, input: depsEdit(base, "api/openapi.yaml", body), want: want,
			}})
		}
	}
}

func TestOpenAPI31PathItems(t *testing.T) {
	t.Parallel()
	base := depsProof(t, "policy.openapi-assertions", "near", "api")
	for _, row := range []struct{ name, before, after string }{
		{"webhooks", "webhooks: {changed: ", "}"},
		{"component paths", "components: {pathItems: {Change: ", "}}"},
	} {
		for _, example := range []string{"", ", example: ok"} {
			var want []string
			if example == "" {
				want = []string{"openapi-policy"}
			}
			body := "openapi: 3.1.0\n" + row.before + "{post: {responses: {'200': {content: {text/plain: {schema: {type: string}" + example + "}}}}}}" + row.after + "\n"
			assertDeps(t, policyfiles.OpenAPIAssertions(), []depsCase{{
				name: row.name + "/" + example, input: depsEdit(base, "api/openapi.yaml", body), want: want,
			}})
		}
	}
	assertDeps(t, policyfiles.OpenAPIAssertions(), []depsCase{{
		name: "webhook extensions", input: depsEdit(base, "api/openapi.yaml", "openapi: 3.1.0\nwebhooks: {x-note: {type: number}}\n"),
	}})
}

func TestOpenAPISchemaReferenceDialectSiblings(t *testing.T) {
	t.Parallel()
	base := depsProof(t, "policy.openapi-assertions", "near", "api")
	for _, version := range []string{"3.0.3", "3.1.0"} {
		for _, siblings := range []string{"type: number, format: float", "properties: {amount: {type: number}}"} {
			var want []string
			if version == "3.1.0" {
				want = []string{"openapi-policy"}
			}
			body := "openapi: " + version + "\ncomponents: {schemas: {Base: {}, Amount: {$ref: '#/components/schemas/Base', " + siblings + "}}}\n"
			assertDeps(t, policyfiles.OpenAPIAssertions(), []depsCase{{
				name: version + "/" + siblings, input: depsEdit(base, "api/openapi.yaml", body), want: want,
			}})
		}
	}
}

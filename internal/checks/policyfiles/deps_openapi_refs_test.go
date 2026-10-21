package policyfiles_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestOpenAPIReferenceLocations(t *testing.T) {
	t.Parallel()
	const red = "Amount: {type: number, format: float}\n"
	const near = "Amount: {type: number, format: double}\n"
	for _, row := range []struct {
		name, reference, file, content string
		want                           []string
	}{
		{"normalized local red", "./nested/../schemas.yaml#/Amount", "api/schemas.yaml", red, []string{"openapi-policy"}},
		{"normalized local near", "./nested/../schemas.yaml#/Amount", "api/schemas.yaml", near, nil},
		{"parent within repository", "../schemas.yaml#/Amount", "schemas.yaml", near, nil},
		{"fragment omitted", "./amount.yaml", "api/amount.yaml", "type: number\nformat: double\n", nil},
		{"empty fragment", "./amount.yaml#", "api/amount.yaml", "type: number\nformat: double\n", nil},
		{"escaped map name", "./schemas.yaml#/amount~1~0value", "api/schemas.yaml", "amount/~value: {type: number, format: double}\n", nil},
		{"percent encoded name", "./schemas.yaml#/amount%20value", "api/schemas.yaml", "amount value: {type: number, format: double}\n", nil},
		{"sequence index", "./schemas.yaml#/schemas/0", "api/schemas.yaml", "schemas: [{type: number, format: float}]\n", []string{"openapi-policy"}},
	} {
		assertDeps(t, policyfiles.OpenAPIAssertions(), []depsCase{{
			name: row.name, input: depsEdit(openapiReferenceFixture(t, row.reference), row.file, row.content), want: row.want,
		}})
	}
}

func TestOpenAPIReferencedDocumentDialects(t *testing.T) {
	t.Parallel()
	base := depsProof(t, "policy.openapi-assertions", "near", "api")
	for _, row := range []struct{ name, document, pointer, reason string }{
		{"3.1", "openapi: 3.1.0\ncomponents: {schemas: {Amount: {type: number, format: double}}}\n", "/components/schemas/Amount", ""},
		{"Swagger", "swagger: '2.0'\ndefinitions: {Amount: {type: number, format: double}}\n", "/definitions/Amount", "Swagger"},
		{"schema named openapi", "openapi: {type: number, format: double}\n", "/openapi", ""},
	} {
		input := depsEdit(base, "api/openapi.yaml", "openapi: 3.0.3\ncomponents: {schemas: {Amount: {$ref: './schemas.yaml#"+row.pointer+"'}}}\n")
		input = depsEdit(input, "api/schemas.yaml", row.document)
		status := model.StatusMeasured
		if row.reason != "" {
			status = model.StatusUnmeasured
		}
		assertDeps(t, policyfiles.OpenAPIAssertions(), []depsCase{{
			name: row.name, input: input, status: status, reason: row.reason, outside: row.reason != "",
		}})
	}
}

func TestOpenAPIReferenceChains(t *testing.T) {
	t.Parallel()
	base := openapiReferenceFixture(t, "./schemas/first.yaml#/Amount")
	base = depsEdit(base, "api/schemas/first.yaml", "Amount: {$ref: '../shared/second.yaml#/Amount'}\n")
	near := depsEdit(base, "api/shared/second.yaml", "Amount: {type: number, format: double}\n")
	red := depsEdit(base, "api/shared/second.yaml", "Amount: {type: number, format: float}\n")
	cycle := depsEdit(base, "api/shared/second.yaml", "Amount: {$ref: '../schemas/first.yaml#/Amount'}\n")
	branch := depsEdit(base, "api/shared/second.yaml", "Amount:\n  type: object\n  properties:\n"+
		"    next: {$ref: '../schemas/first.yaml#/Amount'}\n    amount: {type: number, format: float}\n")
	assertDeps(t, policyfiles.OpenAPIAssertions(), []depsCase{
		{name: "multi hop relative base near", input: near},
		{name: "multi hop relative base red", input: red, want: []string{"openapi-policy"}},
		{name: "cross file cycle terminates", input: cycle},
		{name: "cycle retains sibling findings", input: branch, want: []string{"openapi-policy"}},
		{name: "missing second hop", input: base, status: model.StatusUnmeasured, reason: "api/shared/second.yaml"},
	})
}

func TestOpenAPIReferenceLimits(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, reference, reason string
		outside                 bool
	}{
		{"missing fragment target", "./schemas.yaml#/Other", "api/schemas.yaml#/Other", false},
		{"out of range index", "./schemas.yaml#/schemas/2", "api/schemas.yaml#/schemas/2", false},
		{"negative index", "./schemas.yaml#/schemas/-1", "api/schemas.yaml#/schemas/-1", false},
		{"noncanonical index", "./schemas.yaml#/schemas/00", "api/schemas.yaml#/schemas/00", false},
		{"invalid pointer escape", "./schemas.yaml#/Amount~2", "invalid JSON pointer", true},
		{"unterminated pointer escape", "./schemas.yaml#/Amount~", "invalid JSON pointer", true},
		{"plain name fragment", "./schemas.yaml#Amount", "unsupported reference fragment", true},
		{"remote https", "https://example.test/schema.yaml#/Amount", "remote reference", true},
		{"remote authority", "//example.test/schema.yaml#/Amount", "remote reference", true},
		{"remote scheme", "file:///schema.yaml#/Amount", "remote reference", true},
		{"repository escape", "../../schema.yaml#/Amount", "leaves the repository", true},
		{"encoded repository escape", "%2e%2e/%2e%2e/schema.yaml#/Amount", "leaves the repository", true},
		{"absolute file path", "/schema.yaml#/Amount", "unsupported reference path", true},
		{"query changes evidence", "./schemas.yaml?version=1#/Amount", "unsupported reference path", true},
		{"windows path", "..\\schemas.yaml#/Amount", "unsupported reference path", true},
	} {
		input := depsEdit(openapiReferenceFixture(t, row.reference), "api/schemas.yaml",
			"Amount: {type: number, format: double}\nschemas: [{type: number, format: double}]\n")
		assertDeps(t, policyfiles.OpenAPIAssertions(), []depsCase{{
			name: row.name, input: input, status: model.StatusUnmeasured, reason: row.reason, outside: row.outside,
		}})
	}
}

func TestOpenAPIExampleReferencesAreOpaque(t *testing.T) {
	t.Parallel()
	const response = "openapi: 3.0.3\npaths:\n  /amount:\n    get:\n      responses:\n" +
		"        '200':\n          description: amount\n          content:\n            application/json:\n"
	base := depsProof(t, "policy.openapi-assertions", "near", "go", "api")
	for _, row := range []struct{ name, content string }{
		{"singular payload", "              example: {$ref: 'https://example.test/example.yaml'}\n"},
		{"named reference", "              examples:\n                amount: {$ref: 'https://example.test/example.yaml'}\n"},
		{"named value", "              examples:\n                amount:\n                  value: {$ref: '../../absent.yaml'}\n"},
	} {
		assertDeps(t, policyfiles.OpenAPIAssertions(), []depsCase{{
			name: row.name, input: depsEdit(base, "api/openapi.yaml", response+row.content),
		}})
	}
}

func openapiReferenceFixture(t *testing.T, reference string) fixture {
	t.Helper()
	base := depsProof(t, "policy.openapi-assertions", "near", "go", "api")
	return depsEdit(base, "api/openapi.yaml", "openapi: 3.0.3\npaths: {}\ncomponents:\n  schemas:\n"+
		"    Amount: {$ref: '"+reference+"'}\n")
}

func TestOpenAPIMissingReferenceReasons(t *testing.T) {
	t.Parallel()
	for _, row := range []struct{ name, content, target string }{
		{"file", "", "api/schemas.yaml"},
		{"pointer", "Other: {type: string}\n", "api/schemas.yaml#/Amount"},
	} {
		input := depsEdit(openapiReferenceFixture(t, "./schemas.yaml#/Amount"), "api/schemas.yaml", row.content)
		assertDeps(t, policyfiles.OpenAPIAssertions(), []depsCase{{
			name: row.name, input: input, status: model.StatusUnmeasured,
			reason: "api/openapi.yaml:5: $ref ./schemas.yaml#/Amount names " + row.target + ", which the repository does not hold",
		}})
	}
}

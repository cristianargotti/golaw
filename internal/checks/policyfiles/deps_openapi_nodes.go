package policyfiles

import (
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	apiValidation    = "no-invalid-media-type-examples"
	apiError         = "error"
	apiParentPrefix  = "../"
	apiSchema        = "schema"
	apiResponseMedia = "response-media"
	apiType          = "type"
	apiNumber        = "number"
	apiFormatPointer = "/format"
	apiPath          = "path"
	apiParameter     = "parameter"
	apiRequest       = "request"
	apiCallback      = "callback"
	apiContent       = "content"
	apiMedia         = "media"
	apiResponse      = "response"
	apiSeverity      = "severity"
	apiSubject       = "subject"
	apiProperty      = "property"
	apiAssertions    = "assertions"
	apiConst         = "const"
	apiDefined       = "defined"
)

type apiEdge struct {
	key, role string
	many      byte
}

// walk visits schemas, operations, responses and media types, never examples,
// descriptions or extensions, which are payloads; a boolean schema of 3.1
// holds no type, and each file, pointer and role is visited once so a
// recursive schema ends.
func (r *openapiReader) walk(at apiLocation, role string) error {
	r.failed = at.file
	at.node = resolve(at.node)
	if at.node == nil || (role == apiSchema && at.node.Tag == "!!bool") {
		return nil
	}
	visit, active := apiVisit{at.file, at.pointer, role}, apiActive{at.node, role}
	if r.seen[visit] || r.active[active] {
		return nil
	}
	r.seen[visit], r.active[active] = true, true
	defer delete(r.active, active)
	if at.node.Kind != yaml.MappingNode {
		return outside(at.file, at.node.Line, "a nonmapping OpenAPI "+role)
	}
	if ref := child(at.node, "$ref"); ref != nil {
		return r.walkReference(at, role, ref)
	}
	if err := r.inspect(at, role); err != nil {
		return err
	}
	return r.followEdges(at, apiEdges(role))
}

func (r *openapiReader) followEdges(at apiLocation, edges []apiEdge) error {
	for _, edge := range edges {
		next, err := at.branch(edge.key)
		if err != nil {
			return err
		}
		if next.node == nil {
			continue
		}
		if followErr := r.follow(next, edge); followErr != nil {
			return followErr
		}
	}
	return nil
}

func (r *openapiReader) follow(at apiLocation, edge apiEdge) error {
	if edge.many == 0 {
		return r.walk(at, edge.role)
	}
	r.failed = at.file
	if edge.many == 's' {
		return r.sequence(at, edge.role)
	}
	if at.node.Kind != yaml.MappingNode {
		return outside(at.file, at.node.Line, "nonmapping "+at.pointer)
	}
	for _, item := range entries(at.node) {
		if edge.many == 'x' && strings.HasPrefix(item.key, "x-") {
			continue
		}
		if err := r.walk(at.descend(item.key), edge.role); err != nil {
			return err
		}
	}
	return nil
}

func (r *openapiReader) sequence(at apiLocation, role string) error {
	if at.node.Kind != yaml.SequenceNode {
		return outside(at.file, at.node.Line, "nonsequence "+at.pointer)
	}
	for index, node := range at.node.Content {
		next := apiLocation{file: at.file, pointer: at.pointer + "/" + strconv.Itoa(index), node: node}
		if err := r.walk(next, role); err != nil {
			return err
		}
	}
	return nil
}

func (at apiLocation) descend(key string) apiLocation {
	return apiLocation{file: at.file, pointer: at.pointer + "/" + apiPointerPart(key), node: child(at.node, key)}
}

func apiEdges(role string) []apiEdge {
	switch role {
	case "document":
		return apiDocumentEdges()
	case apiPath:
		return apiPathEdges()
	case "operation":
		return []apiEdge{
			{"parameters", apiParameter, 's'},
			{"requestBody", apiRequest, 0},
			{"responses", apiResponse, 'x'},
			{"callbacks", apiCallback, 'm'},
		}
	case apiResponse:
		return []apiEdge{{apiContent, apiResponseMedia, 'm'}, {"headers", apiParameter, 'm'}}
	case apiRequest:
		return []apiEdge{{apiContent, apiMedia, 'm'}}
	case apiParameter:
		return []apiEdge{{apiSchema, apiSchema, 0}, {apiContent, apiMedia, 'm'}}
	case apiMedia, apiResponseMedia:
		return []apiEdge{{apiSchema, apiSchema, 0}}
	case apiSchema:
		return apiSchemaEdges()
	case apiCallback:
		return []apiEdge{{"", apiPath, 'x'}}
	}
	return nil
}

func apiSchemaEdges() []apiEdge {
	return []apiEdge{
		{"properties", apiSchema, 'm'},
		{"items", apiSchema, 0},
		{"additionalProperties", apiSchema, 0},
		{"allOf", apiSchema, 's'},
		{"oneOf", apiSchema, 's'},
		{"anyOf", apiSchema, 's'},
		{"not", apiSchema, 0},
		{"prefixItems", apiSchema, 's'},
		{"$defs", apiSchema, 'm'},
		{"dependentSchemas", apiSchema, 'm'},
		{"patternProperties", apiSchema, 'm'},
		{"contains", apiSchema, 0},
		{"if", apiSchema, 0},
		{"then", apiSchema, 0},
		{wordElse, apiSchema, 0},
		{"propertyNames", apiSchema, 0},
		{"unevaluatedItems", apiSchema, 0},
		{"unevaluatedProperties", apiSchema, 0},
		{"contentSchema", apiSchema, 0},
	}
}

func apiDocumentEdges() []apiEdge {
	return []apiEdge{
		{"paths", apiPath, 'x'},
		{"webhooks", apiPath, 'x'},
		{"components/pathItems", apiPath, 'm'},
		{"components/schemas", apiSchema, 'm'},
		{"components/responses", apiResponse, 'm'},
		{"components/parameters", apiParameter, 'm'},
		{"components/requestBodies", apiRequest, 'm'},
		{"components/headers", apiParameter, 'm'},
		{"components/callbacks", apiCallback, 'm'},
	}
}

func apiPathEdges() []apiEdge {
	methods := []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}
	edges := make([]apiEdge, 1, len(methods)+1)
	edges[0] = apiEdge{"parameters", apiParameter, 's'}
	for _, method := range methods {
		edges = append(edges, apiEdge{method, "operation", 0})
	}
	return edges
}

func (r *openapiReader) inspect(at apiLocation, role string) error {
	switch role {
	case apiSchema:
		return r.number(at)
	case apiResponseMedia:
		if child(at.node, "example") != nil {
			return nil
		}
		examples := child(at.node, "examples")
		if examples != nil && examples.Kind != yaml.MappingNode {
			return outside(at.file, examples.Line, "response examples that is not a named-example mapping")
		}
		if len(entries(examples)) == 0 {
			message := "add example or a nonempty examples mapping at " + at.pointer + " in " + at.file
			r.addIssues(at, r.api.responseRules, message)
		}
	}
	return nil
}

// number judges a scalar type alone: the configured const: number assertion
// does not match a type list, so a list is no number schema here either.
func (r *openapiReader) number(at apiLocation) error {
	kind := child(at.node, apiType)
	if kind != nil && kind.Kind != yaml.ScalarNode {
		if kind.Kind == yaml.SequenceNode {
			return nil
		}
		return outside(at.file, kind.Line, "nonliteral schema type")
	}
	if kind == nil || kind.Value != apiNumber {
		return nil
	}
	format := child(at.node, "format")
	if format != nil && format.Kind != yaml.ScalarNode {
		return outside(at.file, format.Line, "nonliteral number format")
	}
	if format == nil || format.Value != "double" {
		message := "set format: double for the number schema at " + at.pointer + " in " + at.file
		at.pointer += apiFormatPointer
		at.node = kind
		if format != nil {
			at.node = format
		}
		r.addIssues(at, r.api.numberRules, message)
	}
	return nil
}

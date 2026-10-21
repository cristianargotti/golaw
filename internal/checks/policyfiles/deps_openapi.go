package policyfiles

import (
	"context"
	"regexp"
	"slices"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/model"
)

const (
	redoclyFile       = "redocly.yaml"
	redoclyIgnoreFile = ".redocly.lint-ignore.yaml"
	apiVersionKey     = "openapi"
)

var openapiVersion = regexp.MustCompile(`^3\.[01]\.\d+$`)

type openapiAssertions struct{}

type openapiIssue struct {
	finding       model.Finding
	rule, pointer string
}

// OpenAPIAssertions keeps the configured assertions and the contract under the same policy.
func OpenAPIAssertions() model.Check { return openapiAssertions{} }

func (openapiAssertions) Spec() model.Spec { return spec("openapi-assertions", ruleContract, "api") }

func (openapiAssertions) Run(_ context.Context, in model.Input) (model.Result, error) {
	data, found, err := read(in, redoclyFile)
	if err != nil {
		return answer(redoclyFile, err), nil
	}
	result := model.Result{Status: model.Measured(), Scope: redoclyFile}
	if !found {
		result.Findings = []model.Finding{openapiFinding(redoclyFile, 0,
			"add redocly.yaml with error-level number-double, response-example and example-validation rules; init writes it under the api capability")}
		return result, nil
	}
	apis, findings, err := readRedocly(data)
	if err != nil {
		return answer(redoclyFile, err), nil
	}
	issues, failed, err := inspectOpenAPIs(in, apis)
	if err != nil {
		return answer(failed, err), nil
	}
	filtered, err := redoclyIgnore(in, issues)
	if err != nil {
		return answer(redoclyIgnoreFile, err), nil
	}
	result.Findings = slices.Concat(findings, filtered)
	return result, nil
}

// inspectOpenAPIs skips a contract that does not exist yet, because a newly
// rendered repository holds its configuration before its first contract.
func inspectOpenAPIs(in model.Input, apis []redoclyAPI) ([]openapiIssue, string, error) {
	documents := make(map[string]*yaml.Node)
	var issues []openapiIssue
	for _, api := range apis {
		reader := newOpenAPIReader(in, api, documents)
		root, found, err := reader.document(api.root)
		if err != nil {
			return nil, reader.failed, err
		}
		if !found {
			continue
		}
		if err := openapiDialect(api.root, root); err != nil {
			return nil, api.root, err
		}
		if err := reader.walk(apiLocation{file: api.root, pointer: "#", node: root}, "document"); err != nil {
			return nil, reader.failed, err
		}
		issues = append(issues, reader.issues...)
	}
	return issues, "", nil
}

func openapiDialect(file string, root *yaml.Node) error {
	if swagger := child(root, "swagger"); swagger != nil {
		return outside(file, swagger.Line, "Swagger "+swagger.Value+"; use OpenAPI 3.0 or 3.1")
	}
	version := child(root, apiVersionKey)
	if version == nil || version.Kind != yaml.ScalarNode || !openapiVersion.MatchString(version.Value) {
		return outside(file, lineOf(version), "unsupported OpenAPI document dialect; use OpenAPI 3.0 or 3.1")
	}
	return nil
}

func openapiFinding(file string, line int, message string) model.Finding {
	return finding(file, line, "openapi-policy", message)
}

// redoclyRule recognizes the two assertion forms the kit's template writes.
// Another rule on the same subjects reports problems of its own and weakens
// neither, so its limit counts only while a required kind has no rule.
func redoclyRule(node *yaml.Node) (string, bool, error) {
	kind := ""
	subject := child(node, apiSubject)
	if redoclyValue(subject, apiType) == "Schema" {
		kind = apiNumber
	}
	if redoclyValue(subject, apiType) == "MediaType" {
		kind = apiResponse
	}
	if kind == "" {
		return "", false, nil
	}
	if (kind == apiNumber && redoclyValue(subject, apiProperty) != "format") ||
		(kind == apiResponse && child(subject, apiProperty) != nil) {
		return kind, false, outside(redoclyFile, subject.Line, "unsupported assertion subject expression")
	}
	where := child(node, "where")
	if !redoclyFields(node, apiSubject, "where", apiAssertions, apiSeverity, "message", "suggest", "documentationLink") ||
		!redoclyFields(subject, apiType, apiProperty) || where == nil || where.Kind != yaml.SequenceNode || len(where.Content) != 1 {
		return kind, false, outside(redoclyFile, node.Line, "unsupported assertion expression")
	}
	filter := resolve(where.Content[0])
	if !redoclyFilter(filter, kind) {
		return kind, false, outside(redoclyFile, filter.Line, "unsupported where expression")
	}
	valid, known := redoclyAssertion(child(node, apiAssertions), kind)
	if !known {
		return kind, false, outside(redoclyFile, node.Line, "unsupported assertions")
	}
	return kind, valid && redoclyError(node), nil
}

func redoclyEffectiveRule(rule entry) (string, bool, error) {
	kind, valid, err := redoclyRule(rule.value)
	if rule.key == apiValidation {
		kind, valid = "validation", redoclyError(rule.value)
	}
	if err == nil && kind != "" {
		err = redoclySeverity(rule.value)
	}
	return kind, valid, err
}

func redoclyFilter(node *yaml.Node, kind string) bool {
	if !redoclyFields(node, apiSubject, apiAssertions) {
		return false
	}
	subject, assertions := child(node, apiSubject), child(node, apiAssertions)
	if kind == apiNumber {
		return redoclyFields(subject, apiType, apiProperty) && redoclyValue(subject, apiType) == "Schema" &&
			redoclyValue(subject, apiProperty) == apiType && redoclyFields(assertions, apiConst) && redoclyValue(assertions, apiConst) == apiNumber
	}
	defined, known := redoclyBoolean(assertions, apiDefined)
	return redoclyFields(subject, apiType) && redoclyValue(subject, apiType) == "Response" &&
		redoclyFields(assertions, apiDefined) && known && defined
}

func redoclyAssertion(node *yaml.Node, kind string) (bool, bool) {
	if kind == apiNumber {
		defined, known := redoclyBoolean(node, apiDefined)
		return defined && redoclyValue(node, apiConst) == "double", known && redoclyFields(node, apiDefined, apiConst)
	}
	values, ok := redoclyStrings(child(node, "requireAny"))
	return ok && len(values) != 0 && !slices.ContainsFunc(values, func(value string) bool {
		return value != "example" && value != "examples"
	}), redoclyFields(node, "requireAny") && ok
}

func redoclyBoolean(node *yaml.Node, key string) (bool, bool) {
	value := child(node, key)
	if value == nil {
		return false, true
	}
	return value.Value == yamlTrue, value.Kind == yaml.ScalarNode && value.Tag == "!!bool"
}

func redoclyFields(node *yaml.Node, keys ...string) bool {
	if node == nil || node.Kind != yaml.MappingNode {
		return false
	}
	for _, item := range entries(node) {
		if !slices.Contains(keys, item.key) {
			return false
		}
	}
	return true
}

func redoclyValue(node *yaml.Node, key string) string {
	value := child(node, key)
	if value == nil || value.Kind != yaml.ScalarNode {
		return ""
	}
	return value.Value
}

func redoclyStrings(node *yaml.Node) ([]string, bool) {
	node = resolve(node)
	if node == nil || node.Kind != yaml.SequenceNode {
		return nil, false
	}
	values := make([]string, 0, len(node.Content))
	for _, item := range node.Content {
		item = resolve(item)
		if item.Kind != yaml.ScalarNode || item.Tag != dependabotStringTag {
			return nil, false
		}
		values = append(values, item.Value)
	}
	return values, true
}

func redoclyAssertionLimit(api *redoclyAPI, limits map[string]error) error {
	if len(api.numberRules) == 0 && limits[apiNumber] != nil {
		return limits[apiNumber]
	}
	if len(api.responseRules) == 0 && limits[apiResponse] != nil {
		return limits[apiResponse]
	}
	return limits["validation"]
}

func redoclyRepair(key, kind string) string {
	if kind == "validation" {
		return key + ": error"
	}
	assertion := "requireAny: [example, examples]"
	if kind == apiNumber {
		assertion = "defined: true, const: double"
	}
	return key + ": {severity: error, assertions: {" + assertion + "}}"
}

func (r *openapiReader) addIssues(at apiLocation, rules []string, message string) {
	for _, rule := range rules {
		r.issues = append(r.issues, openapiIssue{
			finding: openapiFinding(at.file, at.node.Line, message), rule: rule, pointer: at.pointer,
		})
	}
}

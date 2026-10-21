package policyfiles

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/model"
)

type redoclyAPI struct {
	root                       string
	numberRules, responseRules []string
}

// readRedocly reads the subset of redocly.yaml the check judges: extends of
// the recommended rulesets, no plugins, literal local API roots, and the
// global rules with the rules of each API over them by name; without apis
// the global rules apply to api/openapi.yaml, the contract golaw.mk lints.
func readRedocly(data []byte) ([]redoclyAPI, []model.Finding, error) {
	root, err := decodeYAML(redoclyFile, data)
	if err != nil {
		return nil, nil, err
	}
	if err := redoclyInheritance(root); err != nil {
		return nil, nil, err
	}
	apis := child(root, "apis")
	if apis != nil && apis.Kind != yaml.MappingNode {
		return nil, nil, outside(redoclyFile, apis.Line, "apis that are not a mapping")
	}
	if apis == nil || len(entries(apis)) == 0 {
		apis = &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{{Value: "default"}, {
			Kind:    yaml.MappingNode,
			Content: []*yaml.Node{{Value: "root"}, {Kind: yaml.ScalarNode, Tag: dependabotStringTag, Value: "api/openapi.yaml"}},
		}}}
	}
	var out []redoclyAPI
	var findings []model.Finding
	for _, api := range entries(apis) {
		item, repairs, err := redoclyConfiguredAPI(root, api)
		if err != nil {
			return nil, nil, err
		}
		out, findings = append(out, item), append(findings, repairs...)
	}
	return out, findings, nil
}

func redoclyInheritance(node *yaml.Node) error {
	if inherited := child(node, "extends"); inherited != nil {
		values, ok := redoclyStrings(inherited)
		if !ok || slices.ContainsFunc(values, func(value string) bool {
			return value != "recommended" && value != "recommended-strict"
		}) {
			return outside(redoclyFile, inherited.Line, "unsupported extends "+strings.Join(scalars(inherited), ","))
		}
	}
	if plugins := child(node, "plugins"); plugins != nil && (plugins.Kind != yaml.SequenceNode || len(plugins.Content) != 0) {
		return outside(redoclyFile, plugins.Line, "plugins affecting assertion rules")
	}
	return nil
}

func redoclyConfiguredAPI(root *yaml.Node, api entry) (redoclyAPI, []model.Finding, error) {
	var out redoclyAPI
	if err := redoclyInheritance(api.value); err != nil {
		return out, nil, err
	}
	location := child(api.value, "root")
	if location == nil || location.Tag != dependabotStringTag || !redoclyPath(location.Value) {
		return out, nil, outside(redoclyFile, api.line, "nonliteral or nonlocal API root for "+api.key)
	}
	out.root = path.Clean(location.Value)
	rules, err := redoclyRules(child(root, "rules"), child(api.value, "rules"))
	if err != nil {
		return out, nil, err
	}
	repairs, err := redoclyAssertions(&out, rules)
	if err != nil {
		return out, nil, err
	}
	if len(out.numberRules) == 0 {
		repairs = append(repairs, "an error-level number Schema assertion with defined format: double")
	}
	if len(out.responseRules) == 0 {
		repairs = append(repairs, "an error-level response MediaType assertion requiring example or examples")
	}
	if len(repairs) == 0 {
		return out, nil, nil
	}
	message := "in redocly.yaml set " + strings.Join(repairs, " and ") + " for API " + api.key
	return out, []model.Finding{openapiFinding(redoclyFile, api.line, message)}, nil
}

func redoclyAssertions(api *redoclyAPI, rules []entry) ([]string, error) {
	var repairs []string
	limits := make(map[string]error)
	for _, rule := range rules {
		rule.value = resolve(rule.value)
		kind, valid, err := redoclyEffectiveRule(rule)
		if err != nil {
			limits[kind] = fmt.Errorf("rule %s: %w", rule.key, err)
			continue
		}
		if kind != "" && !valid {
			repairs = append(repairs, redoclyRepair(rule.key, kind))
		}
		if valid && kind == apiNumber {
			api.numberRules = append(api.numberRules, rule.key)
		}
		if valid && kind == apiResponse {
			api.responseRules = append(api.responseRules, rule.key)
		}
	}
	return repairs, redoclyAssertionLimit(api, limits)
}

func redoclyRules(global, local *yaml.Node) ([]entry, error) {
	for _, node := range []*yaml.Node{global, local} {
		if node != nil && node.Kind != yaml.MappingNode {
			return nil, outside(redoclyFile, node.Line, "rules that are not a mapping")
		}
	}
	rules := entries(global)
	if child(global, apiValidation) == nil {
		rules = append(rules, entry{key: apiValidation})
	}
	for _, rule := range entries(local) {
		rule.value = resolve(rule.value)
		index := slices.IndexFunc(rules, func(item entry) bool { return item.key == rule.key })
		if index < 0 {
			rules = append(rules, rule)
			continue
		}
		base := resolve(rules[index].value)
		if base != nil && base.Kind == yaml.MappingNode && rule.value.Kind == yaml.ScalarNode {
			rule.value = &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{{Kind: yaml.ScalarNode, Value: apiSeverity}, rule.value}}
		}
		if base != nil && rule.value.Kind == yaml.MappingNode && base.Kind == yaml.MappingNode {
			rule.value = redoclyOverlay(base, rule.value)
		}
		rules[index] = rule
	}
	return rules, nil
}

func redoclyOverlay(base, local *yaml.Node) *yaml.Node {
	merged := *base
	merged.Content = slices.Clone(base.Content)
	for _, item := range entries(local) {
		value := resolve(item.value)
		if previous := child(base, item.key); previous != nil && previous.Kind == yaml.MappingNode && value.Kind == yaml.MappingNode {
			combined := *previous
			combined.Content = append(slices.Clone(previous.Content), value.Content...)
			value = &combined
		}
		merged.Content = append(merged.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: item.key}, value)
	}
	return &merged
}

// redoclyIgnore reads the ignore file only while a finding remains, so an
// entry outside its subset limits nothing a verdict does not depend on.
func redoclyIgnore(in model.Input, issues []openapiIssue) ([]model.Finding, error) {
	if len(issues) == 0 {
		return nil, nil
	}
	data, found, err := read(in, redoclyIgnoreFile)
	if err != nil {
		return nil, err
	}
	var root *yaml.Node
	if found {
		root, err = decodeYAML(redoclyIgnoreFile, data)
	}
	var findings []model.Finding
	for index := range issues {
		if !redoclyIgnored(root, &issues[index]) {
			findings = append(findings, issues[index].finding)
		}
	}
	if len(findings) == 0 {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return findings, redoclyIgnoreLimit(root)
}

func redoclyIgnoreLimit(root *yaml.Node) error {
	for _, contract := range entries(root) {
		if !redoclyPath(contract.key) || resolve(contract.value).Kind != yaml.MappingNode {
			return outside(redoclyIgnoreFile, contract.line, "unsupported contract entry "+contract.key)
		}
		for _, rule := range entries(contract.value) {
			if _, ok := redoclyStrings(rule.value); !ok {
				return outside(redoclyIgnoreFile, rule.line, "unsupported rule entry "+contract.key+" "+rule.key)
			}
		}
	}
	return nil
}

func redoclyIgnored(root *yaml.Node, issue *openapiIssue) bool {
	for _, contract := range entries(root) {
		if !redoclyPath(contract.key) || path.Clean(contract.key) != issue.finding.Path {
			continue
		}
		pointers, ok := redoclyStrings(child(contract.value, issue.rule))
		if !ok {
			continue
		}
		if slices.Contains(pointers, issue.pointer) {
			return true
		}
		if strings.HasSuffix(issue.pointer, apiFormatPointer) && slices.Contains(pointers, strings.TrimSuffix(issue.pointer, apiFormatPointer)) {
			return true
		}
	}
	return false
}

func redoclyError(node *yaml.Node) bool {
	if node == nil {
		return false
	}
	if node.Kind == yaml.MappingNode {
		node = child(node, apiSeverity)
		return node == nil || node.Value == apiError
	}
	return node.Tag == dependabotStringTag && node.Value == apiError
}

func redoclySeverity(node *yaml.Node) error {
	if node != nil && node.Kind == yaml.MappingNode {
		node = child(node, apiSeverity)
	}
	if node != nil && (node.Tag != dependabotStringTag || !slices.Contains([]string{apiError, "warn", "off"}, node.Value)) {
		return outside(redoclyFile, node.Line, "unsupported rule severity")
	}
	return nil
}

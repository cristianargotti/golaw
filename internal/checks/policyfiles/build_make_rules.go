package policyfiles

import (
	"slices"
	"strings"
)

func mergeMakeRules(rules []makeRule, target string) (makeRule, bool) {
	var merged makeRule
	found := false
	for _, rule := range rules {
		if rule.condition != "" || !slices.Contains(rule.targets, target) {
			continue
		}
		if !found {
			merged = rule
			merged.prerequisites = nil
			found = true
		}
		merged.prerequisites = append(merged.prerequisites, rule.prerequisites...)
		if rule.recipe != nil {
			merged.recipe = rule.recipe
		}
	}
	return merged, found
}

func (r *makeReader) recipe(index int, text string) {
	if index >= 0 {
		r.result.rules[index].recipe = append(r.result.rules[index].recipe, text)
	}
}

func (r *makeReader) addRule(line makeLine, condition string) (int, error) {
	colon := makeDelimiter(line.text, ":")
	if colon < 0 {
		if strings.HasPrefix(line.text, "$(") || strings.HasPrefix(line.text, "${") {
			r.result.opaque = append(r.result.opaque, makeOpaque(line))
		}
		return -1, nil
	}
	if strings.HasPrefix(line.text[colon:], "::") {
		r.result.double = append(r.result.double, makeRule{
			targets: makeWords(line.text[:colon]), file: line.file, line: line.line, condition: condition,
		})
		return -1, nil
	}
	tail := line.text[colon+1:]
	var recipe []string
	if semi := makeDelimiter(tail, ";"); semi >= 0 {
		recipe = []string{strings.TrimSpace(tail[semi+1:])}
		tail = tail[:semi]
	}
	if pipe := makeDelimiter(tail, "|"); pipe >= 0 {
		tail = tail[:pipe]
	}
	index := len(r.result.rules)
	r.result.rules = append(r.result.rules, makeRule{
		targets: makeWords(line.text[:colon]), prerequisites: makeWords(tail),
		recipe: recipe, file: line.file, line: line.line, condition: condition,
	})
	r.events = append(r.events, makeEvent{index: index})
	return index, nil
}

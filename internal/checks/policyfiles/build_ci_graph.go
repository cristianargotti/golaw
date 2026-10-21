package policyfiles

import (
	"errors"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// ciGraph walks the targets ci reaches, each once, with the walk's active
// path kept apart so a cycle is named instead of followed.
type ciGraph struct {
	ciEvidence

	makefile makefile
	visited  map[string]bool
	active   map[string]bool
}

// ciReachable keeps proven edges beside the first uncertain one, because
// another path or on-demand execution can complete the required coverage.
func ciReachable(m makefile) ([]string, error) {
	graph := ciGraph{makefile: m, visited: make(map[string]bool), active: make(map[string]bool)}
	graph.visit("ci", makeRule{file: buildMakefile})
	graph.add(nil, ciIncludes(m))
	for _, include := range m.missing {
		if include.condition != "" {
			graph.add(nil, outside(include.file, include.line, "conditional include "+include.name))
		}
	}
	return graph.targets, graph.limit
}

// visit records a reached target and every way the reading could have
// missed one of its prerequisites: a raw token, a cycle, a computed or
// conditional definition; each stays deferred until coverage needs it.
func (g *ciGraph) visit(target string, source makeRule) {
	if !ciLiteralTarget(target) || g.active[target] {
		g.add(nil, outside(source.file, source.line, "unresolved or cyclic prerequisite "+target))
		return
	}
	if g.visited[target] {
		return
	}
	g.visited[target], g.active[target] = true, true
	defer delete(g.active, target)
	g.targets = append(g.targets, target)
	g.add(nil, ciDefinitionLimit(g.makefile, target))
	if conditional, found := g.makefile.conditional(target); found {
		g.add(nil, outside(conditional.file, conditional.line,
			"target "+target+" inside the conditional block opened at "+conditional.condition))
	}
	g.edges(target)
}

// edges follows the prerequisites of every unconditional rule line of the
// target and the recipe make keeps, the last one.
func (g *ciGraph) edges(target string) {
	var last makeRule
	for _, rule := range g.makefile.rules {
		if rule.condition != "" || !slices.Contains(rule.targets, target) {
			continue
		}
		for _, prerequisite := range rule.prerequisites {
			g.visit(prerequisite, rule)
		}
		if rule.recipe != nil {
			last = rule
		}
	}
	g.recipes(last, slices.ContainsFunc(buildTargets(), func(check buildTarget) bool { return check.name == target }))
}

func (g *ciGraph) recipes(rule makeRule, carriedCheck bool) {
	for _, line := range rule.recipe {
		if text := strings.TrimSpace(line); text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		targets, recognized := recipeTargets(line)
		// Carrying a named check does not require proving its tool recipe.
		if !recognized && carriedCheck {
			continue
		}
		if !recognized {
			g.add(nil, outside(rule.file, rule.line, "opaque recipe "+line))
			continue
		}
		for _, target := range targets {
			g.visit(target, rule)
		}
	}
}

// ciEvidence gathers proven targets beside the first stated limit, a read
// or parse failure replacing a limit because it is never a stated one.
type ciEvidence struct {
	targets []string
	limit   error
}

// add keeps the targets whatever the error, so a limit never erases the
// evidence read beside it.
func (e *ciEvidence) add(targets []string, err error) {
	e.targets = append(e.targets, targets...)
	if e.limit == nil || err != nil && !errors.Is(err, model.ErrOutsideSubset) {
		e.limit = err
	}
}

// Literal rules prove their own edges but do not exclude additional rules
// with computed names, so only their known definitions leave this query.
func ciDefinitionLimit(m makefile, target string) error {
	rules := make([]makeRule, 0, len(m.rules))
	for _, rule := range m.rules {
		if rule.condition == "" && slices.Contains(rule.targets, target) {
			continue
		}
		rules = append(rules, rule)
	}
	m.rules = rules
	return makeTargetLimit(m, target)
}

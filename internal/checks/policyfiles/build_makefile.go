package policyfiles

import (
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// makefile is a Makefile with its includes resolved, as the build checks read it.
type makefile struct {
	files   []string      // Files read, root first, then includes in read order.
	rules   []makeRule    // Every explicit rule, in read order.
	vars    []makeVar     // Every assignment, including exports, in read order.
	missing []makeInclude // Include lines whose files the snapshot does not hold.
	opaque  []makeOpaque  // Top-level function expressions kept without execution.
	double  []makeRule    // Double-colon rules, their targets alone, outside the rules the checks read.
}

// makeRule preserves an explicit rule and its source for the checks' evidence.
type makeRule struct {
	targets       []string // Targets of the rule line.
	prerequisites []string // Normal prerequisites, with order-only ones left out.
	recipe        []string // Recipe text with the leading tab removed.
	file          string   // Repository-relative source file.
	line          int      // One-based line of the rule.
	condition     string   // Conditional opener's file:line, empty outside a block.
}

// makeVar preserves assignments without evaluating computed values or conditions.
type makeVar struct {
	name      string // Variable name.
	value     string // Raw assignment value, including computed expressions.
	exported  bool   // Whether the variable is exported after this assignment.
	target    string // Target of a target-specific assignment, empty for a global one.
	file      string // Repository-relative source file.
	line      int    // One-based line of the assignment or export.
	condition string // Conditional opener's file:line, empty outside a block.
}

// makeInclude preserves missing includes so checks can distinguish optional ones.
type makeInclude struct {
	name      string // Name written by the include line.
	optional  bool   // Whether the directive is -include or sinclude.
	file      string // Repository-relative source file.
	line      int    // One-based line of the include directive.
	condition string // Conditional opener's file:line, empty outside a block.
}

// makeOpaque preserves unevaluated top-level expressions for dependent checks.
type makeOpaque struct {
	text string // Unevaluated expression text.
	file string // Repository-relative source file.
	line int    // One-based line of the expression.
}

type makeEvent struct {
	index    int
	op       string
	variable bool
}

type makeReader struct {
	in       model.Input
	result   makefile
	active   map[string]bool
	events   []makeEvent
	tainted  map[string]bool
	bindings map[string]makeBinding
}

// readMakefile keeps unsupported expressions for the check that needs them.
// Its subset covers LF/CRLF, blanks, non-recipe comments and continuations,
// literal and unresolved rule tokens, pattern targets, normal and order-only
// prerequisites, opaque tab recipes, literal assignments, export/unexport,
// root-relative literal includes and snapshot globs, unevaluated conditional
// blocks with else chains, define blocks, target-specific assignments and
// opaque top-level functions. Only literal variable references are expanded;
// computed includes and include cycles are stated limits, and a double-colon
// rule is recorded with its targets for the check that needs one of them.
// Expansion preserves a raw token beyond 32 active references or MaxReadBytes
// of work, because even a shallow chain can otherwise grow exponentially.
func readMakefile(in model.Input, path string) (makefile, error) {
	r := makeReader{
		in: in, active: make(map[string]bool), tainted: make(map[string]bool),
		bindings: make(map[string]makeBinding),
	}
	if _, err := r.load(path, ""); err != nil {
		return makefile{}, err
	}
	r.evaluate()
	return r.result, nil
}

// rule answers the unconditional reading of a target, merging prerequisites
// from every rule line and keeping the last recipe that is present.
func (m makefile) rule(target string) (makeRule, bool) {
	return mergeMakeRules(m.rules, target)
}

// conditional answers the first conditional definition of a target, consulted
// by a check only when no unconditional definition exists.
func (m makefile) conditional(target string) (makeRule, bool) {
	for _, rule := range m.rules {
		if rule.condition != "" && slices.Contains(rule.targets, target) {
			return rule, true
		}
	}
	return makeRule{}, false
}

// exported answers the unconditional, global reading of a variable: its last
// assignment's value and whether it is exported.
func (m makefile) exported(name string) (string, bool) {
	var value string
	exported := false
	for _, v := range m.vars {
		if v.name == name && v.condition == "" && v.target == "" {
			value, exported = v.value, v.exported
		}
	}
	return value, exported
}

// conditionalVar answers the first conditional, global definition of a variable,
// consulted by a check only when no unconditional definition exists.
func (m makefile) conditionalVar(name string) (makeVar, bool) {
	for _, v := range m.vars {
		if v.name == name && v.condition != "" && v.target == "" {
			return v, true
		}
	}
	return makeVar{}, false
}

func (r *makeReader) load(file, condition string) (bool, error) {
	data, found, err := read(r.in, file)
	if err != nil || !found {
		return found, err
	}
	r.result.files = append(r.result.files, file)
	r.active[file] = true
	defer delete(r.active, file)
	return true, r.parse(makeLines(file, data), condition)
}

func (r *makeReader) parse(lines []makeLine, inherited string) error {
	block := makeBlock{inherited: inherited}
	current := -1
	for i := 0; i < len(lines); {
		index := i
		line := lines[index]
		i++
		if strings.HasPrefix(line.text, "\t") {
			r.recipe(current, line.text[1:])
			continue
		}
		line.text = strings.TrimSpace(line.text)
		if line.text == "" || block.directive(line) {
			continue
		}
		condition := block.condition()
		if r.define(lines, &index, condition) {
			i = index + 1
			current = -1
			continue
		}
		var err error
		current, err = r.statement(line, condition)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *makeReader) statement(line makeLine, condition string) (int, error) {
	if r.variable(line, condition) {
		return -1, nil
	}
	included, err := r.include(line, condition)
	if err != nil || included {
		return -1, err
	}
	return r.addRule(line, condition)
}

func (r *makeReader) evaluate() {
	r.markTainted()
	for _, event := range r.events {
		if event.variable {
			r.assign(event)
			continue
		}
		rule := &r.result.rules[event.index]
		rule.targets = r.expandWords(rule.targets)
		rule.prerequisites = r.expandWords(rule.prerequisites)
	}
}

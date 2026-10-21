package bootstrap

import (
	"regexp"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/install"
)

var ruleLine = regexp.MustCompile(`^([A-Za-z0-9_][A-Za-z0-9_./-]*(?:[ \t]+[A-Za-z0-9_][A-Za-z0-9_./-]*)*)[ \t]*::?(?:[^:=]|$)`)

// A target-specific assignment is not a rule; inline recipes and comments may contain equals signs.
var ruleAssignment = regexp.MustCompile(`^[^;#]*=`)

// Collision retains the first location so a proposal names the target to repair.
type Collision struct {
	Target string
	Line   int
}

// MakeCollisions names only literal rule targets so uncertain syntax cannot block a proposal.
// A rule with prerequisites only merges with the other file's rule, so only two
// recipes, which make overrides, or mixed : and :: rules, which make refuses, collide.
func MakeCollisions(makefile, included []byte) []Collision {
	shared := make(map[string][]makeRule)
	for _, rule := range makeRules(included) {
		shared[rule.Target] = append(shared[rule.Target], rule)
	}
	var collisions []Collision
	for _, rule := range makeRules(makefile) {
		if slices.ContainsFunc(shared[rule.Target], func(other makeRule) bool {
			return rule.recipe && other.recipe || rule.double != other.double
		}) {
			collisions = append(collisions, rule.Collision)
			delete(shared, rule.Target)
		}
	}
	return collisions
}

type makeRule struct {
	Collision

	recipe, double bool
}

func makeRules(data []byte) []makeRule {
	lines := logicalLines(data)
	byNumber := make(map[int]int, len(lines))
	for index, line := range lines {
		byNumber[line.number] = index
	}
	targets := makeTargets(data)
	rules := make([]makeRule, 0, len(targets))
	for _, target := range targets {
		index := byNumber[target.Line]
		_, body, _ := strings.Cut(lines[index].text, ":")
		rules = append(rules, makeRule{
			Collision: target, double: strings.HasPrefix(body, ":"),
			recipe: makeRecipe(body, lines[index+1:]),
		})
	}
	return rules
}

func makeRecipe(body string, following []logicalLine) bool {
	beforeComment, _, _ := strings.Cut(body, "#")
	if strings.Contains(beforeComment, ";") {
		return true
	}
	for _, line := range following {
		if strings.HasPrefix(line.text, "\t") {
			return true
		}
		text := strings.TrimSpace(line.text)
		if text != "" && !strings.HasPrefix(text, "#") {
			return false
		}
	}
	return false
}

func (plan *TemplatePlan) makeCollisions() []Collision {
	var makefile, kit install.Change
	for _, input := range plan.inputs {
		switch input.Path {
		case "Makefile":
			makefile = input
		case "golaw.mk":
			kit = input
		}
	}
	if !makefile.Existed || kit.Path == "" {
		return nil
	}
	return MakeCollisions(makefile.Before, kit.After)
}

type logicalLine struct {
	text   string
	number int
}

func logicalLines(data []byte) []logicalLine {
	physical := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	lines := make([]logicalLine, 0, len(physical))
	for index := 0; index < len(physical); index++ {
		line := logicalLine{text: physical[index], number: index + 1}
		for strings.HasSuffix(line.text, "\\") && index+1 < len(physical) {
			index++
			line.text = strings.TrimSuffix(line.text, "\\") + " " + strings.TrimSpace(physical[index])
		}
		lines = append(lines, line)
	}
	return lines
}

func makeTargets(data []byte) []Collision {
	var targets []Collision
	depth := 0
	for _, line := range logicalLines(data) {
		if strings.HasPrefix(line.text, "\t") {
			continue
		}
		text := strings.TrimSpace(line.text)
		directive := makeDirective(text)
		switch {
		case directive == "define":
			depth++
		case directive == "endef":
			depth = max(0, depth-1)
		case depth == 0 && strings.HasPrefix(directive, ".RECIPEPREFIX"):
			// A custom prefix makes subsequent recipe lines ambiguous to this tab-only reader.
			return targets
		case depth == 0:
			targets = append(targets, ruleTargets(text, line.number)...)
		}
	}
	return targets
}

func makeDirective(text string) string {
	modified := false
	for word := range strings.FieldsSeq(text) {
		switch word {
		case "export", "override", "private":
			modified = true
		default:
			if modified && word != "define" && !strings.HasPrefix(word, ".RECIPEPREFIX") {
				return ""
			}
			return word
		}
	}
	return ""
}

func ruleTargets(text string, number int) []Collision {
	match := ruleLine.FindStringSubmatch(text)
	_, body, _ := strings.Cut(text, ":")
	if match == nil || ruleAssignment.MatchString(body) {
		return nil
	}
	names := strings.Fields(match[1])
	targets := make([]Collision, 0, len(names))
	for _, name := range names {
		targets = append(targets, Collision{Target: name, Line: number})
	}
	return targets
}

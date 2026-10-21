package policyfiles

import (
	"slices"
	"strings"
)

// owners preserves rule order so the last matching assignment wins.
type owners struct {
	rules []ownerRule
}

// ownerRule keeps the source line and normalized owners so findings cite the assignment.
type ownerRule struct {
	pattern string
	owners  []string
	line    int
}

// readOwners shares bootstrap.AllowedPaths' subset so both readers assign ownership alike:
// catchalls *, **, /** and **/*; rooted and unrooted patterns; directory suffixes;
// ordinary stars and whole-component **; last match; empty owner lists; normalized
// named owners and email owners. Negation, brackets, question wildcards, escapes,
// malformed recursive wildcards, empty or dot path components and unsupported
// owners are outside the subset at their line, so uncertain syntax cannot deny a commit.
// This family cannot import bootstrap; tools/cicheck/m3_owners_agreement_test.go
// pins this copy to the same ownership decisions.
func readOwners(path string, data []byte) (owners, error) {
	var result owners
	lineNumber := 0
	for line := range strings.SplitSeq(string(data), "\n") {
		lineNumber++
		fields := strings.Fields(line)
		if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		rule, err := readOwnerRule(path, fields, lineNumber)
		if err != nil {
			return owners{}, err
		}
		result.rules = append(result.rules, rule)
	}
	return result, nil
}

// foreign compares the assigned owners to the baseline; the int is the selected
// rule's line so findings can cite the base assignment.
func (o owners) foreign(path string) (bool, int) {
	rule := o.assignment(path)
	return len(rule.owners) != 0 && !slices.Equal(rule.owners, o.baseline()), rule.line
}

// assignment keeps the last match so a later empty owner list clears an earlier one.
func (o owners) assignment(path string) ownerRule {
	var matched ownerRule
	for _, rule := range o.rules {
		if ownerMatches(rule.pattern, path) {
			matched = rule
		}
	}
	return matched
}

// baseline uses /* only without a catchall because root files do not cover nested paths.
func (o owners) baseline() []string {
	var global, root []string
	found := false
	for _, rule := range o.rules {
		switch rule.pattern {
		case "*", "**", "/**", "**/*":
			global, found = rule.owners, true
		case "/*":
			root = rule.owners
		}
	}
	if found {
		return global
	}
	return root
}

func readOwnerRule(path string, fields []string, line int) (ownerRule, error) {
	if reason := ownerPatternProblem(fields[0]); reason != "" {
		return ownerRule{}, outside(path, line, reason+": "+fields[0])
	}
	rule := ownerRule{pattern: fields[0], line: line}
	for _, name := range fields[1:] {
		if strings.HasPrefix(name, "#") {
			break
		}
		if !ownerSupported(name) {
			return ownerRule{}, outside(path, line, "unsupported owner: "+name)
		}
		if strings.HasPrefix(name, "@") {
			name = strings.ToLower(name)
		}
		rule.owners = append(rule.owners, name)
	}
	slices.Sort(rule.owners)
	rule.owners = slices.Compact(rule.owners)
	return rule, nil
}

func ownerSupported(owner string) bool {
	if name, named := strings.CutPrefix(owner, "@"); named {
		account, team, hasTeam := strings.Cut(name, "/")
		return ownerNamedPart(account, false) && (!hasTeam || ownerNamedPart(team, true))
	}
	name, domain, found := strings.Cut(owner, "@")
	return found && name != "" && domain != "" && !strings.ContainsAny(name, "/") && !strings.ContainsAny(domain, "@/")
}

func ownerNamedPart(name string, team bool) bool {
	if name == "" {
		return false
	}
	for _, letter := range name {
		if letter >= 'a' && letter <= 'z' || letter >= 'A' && letter <= 'Z' ||
			letter >= '0' && letter <= '9' || letter == '-' || team && letter == '_' {
			continue
		}
		return false
	}
	return true
}

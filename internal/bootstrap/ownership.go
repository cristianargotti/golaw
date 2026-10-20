package bootstrap

import (
	"fmt"
	"slices"
	"strings"
)

// Ownership separates writable destinations from supported foreign assignments.
type Ownership struct {
	Allowed     []string
	Protected   []string
	Unsupported []string
}

type ownershipRule struct {
	pattern string
	owners  []string
}

// AllowedPaths ignores unsupported assignments so uncertain syntax cannot deny adoption.
func AllowedPaths(paths []string, codeowners []byte) (Ownership, error) {
	rules, unsupported := ownershipRules(codeowners)
	result := Ownership{Unsupported: unsupported}
	baseline := ownershipBaseline(rules)
	for _, destination := range paths {
		var owners []string
		for _, rule := range rules {
			if ownershipMatches(rule.pattern, destination) {
				owners = rule.owners
			}
		}
		if len(owners) != 0 && !slices.Equal(owners, baseline) {
			result.Protected = append(result.Protected, destination)
		} else {
			result.Allowed = append(result.Allowed, destination)
		}
	}
	return result, nil
}

// ownershipBaseline falls back to the root files line only without a line
// for every path, because /* assigns no nested file.
func ownershipBaseline(rules []ownershipRule) []string {
	var global, root []string
	found := false
	for _, rule := range rules {
		switch {
		case ownershipCatchall(rule.pattern):
			global, found = rule.owners, true
		case rule.pattern == "/*":
			root = rule.owners
		}
	}
	if found {
		return global
	}
	return root
}

func ownershipCatchall(pattern string) bool {
	switch pattern {
	case "*", "**", "/**", "**/*":
		return true
	default:
		return false
	}
}

func ownershipRules(data []byte) ([]ownershipRule, []string) {
	var rules []ownershipRule
	var unsupported []string
	lineNumber := 0
	for line := range strings.SplitSeq(string(data), "\n") {
		lineNumber++
		fields := strings.Fields(line)
		if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		rule, reason := ownershipAssignment(fields)
		if reason != "" {
			unsupported = append(unsupported, fmt.Sprintf("CODEOWNERS line %d: %s: %s", lineNumber, reason, fields[0]))
			continue
		}
		rules = append(rules, rule)
	}
	return rules, unsupported
}

func ownershipAssignment(fields []string) (ownershipRule, string) {
	if reason := ownershipPatternProblem(fields[0]); reason != "" {
		return ownershipRule{}, reason
	}
	var owners []string
	for _, owner := range fields[1:] {
		if strings.HasPrefix(owner, "#") {
			break
		}
		if !ownershipOwnerSupported(owner) {
			return ownershipRule{}, "unsupported owner"
		}
		if strings.HasPrefix(owner, "@") {
			owner = strings.ToLower(owner)
		}
		owners = append(owners, owner)
	}
	slices.Sort(owners)
	return ownershipRule{pattern: fields[0], owners: slices.Compact(owners)}, ""
}

func ownershipOwnerSupported(owner string) bool {
	if name, named := strings.CutPrefix(owner, "@"); named {
		account, team, hasTeam := strings.Cut(name, "/")
		return ownershipNamedPart(account, false) && (!hasTeam || ownershipNamedPart(team, true))
	}
	name, domain, found := strings.Cut(owner, "@")
	return found && name != "" && domain != "" && !strings.ContainsAny(name, "/") && !strings.ContainsAny(domain, "@/")
}

func ownershipNamedPart(name string, team bool) bool {
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

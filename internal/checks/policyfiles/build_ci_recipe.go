package policyfiles

import "strings"

const ciMakeCommand = "make"

// recipeTargets recognizes only a standalone recursive make invocation,
// because shell execution and computed arguments cannot prove coverage.
func recipeTargets(line string) ([]string, bool) {
	if strings.ContainsAny(line, "\r\n") {
		return nil, false
	}
	words := strings.Fields(strings.TrimPrefix(strings.TrimSpace(line), "@"))
	if len(words) < 2 || words[0] != ciMakeCommand && words[0] != "$(MAKE)" {
		return nil, false
	}
	for _, target := range words[1:] {
		if !ciLiteralTarget(target) {
			return nil, false
		}
	}
	return words[1:], true
}

// ciLiteralTarget accepts a plain target name only: a flag, an assignment,
// a reference or a pattern could name any target, so it proves none.
func ciLiteralTarget(target string) bool {
	if target == "" || strings.HasPrefix(target, "-") {
		return false
	}
	for _, char := range target {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || strings.ContainsRune("_./-", char) {
			continue
		}
		return false
	}
	return true
}

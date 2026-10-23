package policyfiles

import (
	"regexp"
	"slices"
	"strings"
)

const (
	ciMakeCommand = "make"
	ciEnvCommand  = "env"
)

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

// ciMakeRunners are the commands that can start make: make itself, a shell,
// an interpreter or a wrapper that runs its arguments as a command.
const ciMakeRunners = "make gmake sh bash zsh dash ksh mksh ash fish csh tcsh busybox pwsh powershell cmd " +
	"exec eval command builtin source . xargs nohup time sudo doas nice timeout watch find parallel " +
	"python perl ruby node npx npm php lua"

// ciMakeToken finds make or gmake as a token anywhere in a line, a quoted
// argument included, because a container, a remote host or a login shell
// may run the quoted command, so such a line can start make.
var ciMakeToken = regexp.MustCompile(`(^|[^A-Za-z0-9_.-])g?make([^A-Za-z0-9_.-]|$)`)

// recipeRunsNoMake answers whether a recipe line certainly starts no make:
// after its @, - and + prefixes, an env word and NAME=value assignments, its
// command is a literal program outside ciMakeRunners, and no shell operator,
// command substitution, $(shell) or make token could start one beside it.
func recipeRunsNoMake(line string) bool {
	text := strings.TrimLeft(strings.TrimSpace(line), "@-+")
	for _, starter := range []string{";", "&", "|", "`", "\r", "\n", "$$(", "<(", ">(", "$(shell", "${shell", "MAKE"} {
		if strings.Contains(text, starter) {
			return false
		}
	}
	words := strings.Fields(text)
	if len(words) > 0 && words[0] == ciEnvCommand {
		words = words[1:]
	}
	for len(words) > 0 && ciAssignment(words[0]) {
		words = words[1:]
	}
	if len(words) == 0 || !ciLiteralProgram(words[0]) || ciMakeToken.MatchString(text) {
		return false
	}
	return !slices.Contains(strings.Fields(ciMakeRunners), strings.TrimRight(words[0], "0123456789."))
}

// ciAssignment recognizes a NAME=value word, an environment assignment the
// shell applies to the command after it.
func ciAssignment(word string) bool {
	name, _, found := strings.Cut(word, "=")
	return found && name != "" && strings.TrimLeft(name, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_") == "" &&
		(name[0] < '0' || name[0] > '9')
}

// ciLiteralProgram accepts a program named by a plain word: a path names a
// script and a reference or a quote a computed command, either may run make.
func ciLiteralProgram(word string) bool {
	return word != "" && word[0] != '-' && strings.Trim(word, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._+-") == ""
}

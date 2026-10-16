package shell

import (
	"regexp"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// hooksPath matches the configuration override that disarms the hooks.
var hooksPath = regexp.MustCompile(`(?i)^core\.hookspath=`)

// noVerifyFlag is the long form of the bypass every guarded subcommand takes.
const noVerifyFlag = "--no-verify"

// The short commit options that take a value: the first set takes the rest
// of its cluster or, at the end of the cluster, the next word; the second
// set takes an attached value only, so -uno is one option and carries no -n.
const (
	shortWithValue     = "mFCct"
	shortOptionalValue = "uS"
)

// NoVerify is shell.no-verify: no command skips the git hooks, neither with
// the flag (--no-verify, and -n on commit) nor with a hooks path override.
func NoVerify() model.Check {
	return commandRule{spec: spec("shell.no-verify"), judge: judgeNoVerify}
}

func judgeNoVerify(_ model.Input, segs []segment) ([]model.Finding, error) {
	for _, call := range gitCalls(segs) {
		bypass := call.sub == subCommit && commitSkipsHooks(call.args) ||
			(call.sub == "merge" || call.sub == "rebase" || call.sub == subPush) && call.has(noVerifyFlag) ||
			slices.ContainsFunc(call.words, hooksPath.MatchString)
		if bypass {
			return []model.Finding{deny("no-verify", "the command skips the git hooks; the hooks are the rule")}, nil
		}
	}
	return nil, nil
}

// commitSkipsHooks reads the arguments of a commit the way git parses them:
// the value of an option is never an option, so -m --no-verify is a
// message; a short cluster carries -n up to its first option that takes a
// value, so -nm skips the hooks; and the words after -- are paths.
func commitSkipsHooks(args []string) bool {
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--":
			return false
		case a == noVerifyFlag:
			return true
		case takesValue(a):
			i++
		case len(a) > 1 && a[0] == '-' && a[1] != '-':
			skips, next := cluster(a[1:])
			if skips {
				return true
			}
			if next {
				i++
			}
		}
	}
	return false
}

// cluster reads a short option cluster letter by letter: it reports whether
// an n comes before the first letter that takes a value, and whether that
// letter ends the cluster and so takes the next word as its value.
func cluster(letters string) (bool, bool) {
	for i, r := range letters {
		switch {
		case r == 'n':
			return true, false
		case strings.ContainsRune(shortWithValue, r):
			return false, i == len(letters)-1
		case strings.ContainsRune(shortOptionalValue, r):
			return false, false
		}
	}
	return false, false
}

// takesValue reports whether a long commit option reads the next word as
// its value when no = joins the two.
func takesValue(option string) bool {
	switch option {
	case "--message", "--file", "--reuse-message", "--reedit-message", "--template", "--author",
		"--date", "--cleanup", "--fixup", "--squash", "--trailer", "--pathspec-from-file":
		return true
	default:
		return false
	}
}

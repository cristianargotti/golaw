package shell

import (
	"regexp"
	"slices"
	"strings"
)

// messageFlag is a short option cluster that ends in m (-m, -am), whose
// next word is a commit message.
var messageFlag = regexp.MustCompile(`^-[a-zA-Z]*m$`)

// messagesOf returns the message parts a commit call carries through -m
// (also inside a short cluster such as -am), --message <text> and
// --message=<text>, in order; a message file (-F) is not read here because
// the commit rung of M2 judges it.
func messagesOf(call gitCall) []string {
	var out []string
	for i := 0; i < len(call.args); i++ {
		switch a := call.args[i]; {
		case messageFlag.MatchString(a) || a == "--message":
			if i+1 < len(call.args) {
				out = append(out, call.args[i+1])
				i++
			}
		case strings.HasPrefix(a, "--message="):
			out = append(out, strings.TrimPrefix(a, "--message="))
		}
	}
	return out
}

// messageOf joins the parts the way git does: one blank line between them.
func messageOf(call gitCall) string { return strings.Join(messagesOf(call), "\n\n") }

// matchesArg reports whether an argument of the call matches the pattern.
func matchesArg(call gitCall, pattern *regexp.Regexp) bool {
	return slices.ContainsFunc(call.args, pattern.MatchString)
}

// isCommit reports whether the call creates a commit; an amend rewrites one
// and is not judged by the commit laws of a hook.
func isCommit(call gitCall) bool { return call.sub == subCommit && !call.has("--amend") }

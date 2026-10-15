package shell

import (
	"regexp"
	"slices"
	"strings"
)

// assignment is the shape of a variable assignment in front of a program.
var assignment = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)

// The subcommands the rules name more than once.
const (
	subCommit = "commit"
	subPush   = "push"
)

// gitCall is one git invocation of a command line: the subcommand, its
// arguments and every word after the wrappers.
type gitCall struct {
	sub   string
	args  []string
	words []string
}

// gitCalls lists the git invocations among the segments.
func gitCalls(segs []segment) []gitCall {
	var out []gitCall
	for _, seg := range segs {
		if call, ok := gitCommand(seg); ok {
			out = append(out, call)
		}
	}
	return out
}

// gitCommand reads a git segment, seeing through the wrappers and the
// global options such as -C <dir> and -c <key=value>.
func gitCommand(seg segment) (gitCall, bool) {
	words := unwrap(seg)
	if len(words) == 0 || base(words[0]) != "git" {
		return gitCall{}, false
	}
	args := words[1:]
	for len(args) > 0 && strings.HasPrefix(args[0], "-") {
		if (args[0] == "-C" || args[0] == "-c") && len(args) > 1 {
			args = args[2:]
			continue
		}
		args = args[1:]
	}
	if len(args) == 0 {
		return gitCall{}, false
	}
	return gitCall{sub: args[0], args: args[1:], words: words}, true
}

// unwrap drops the leading variable assignments and the wrappers that run
// their argument as the command (env, sudo, command, exec, nohup, time,
// nice, timeout with its duration), so a rule sees the program; a wrapper
// with nothing after it is the program itself.
func unwrap(seg segment) []string {
	words := []string(seg)
	for len(words) > 0 {
		switch {
		case assignment.MatchString(words[0]):
			words = words[1:]
		case base(words[0]) == "timeout" && len(words) > 1:
			words = words[2:]
		case isWrapper(words[0]) && len(words) > 1:
			words = words[1:]
		default:
			return words
		}
	}
	return words
}

// isWrapper reports whether a program runs its arguments as the command.
func isWrapper(name string) bool {
	switch base(name) {
	case "env", "sudo", "doas", "command", "exec", "nohup", "time", "nice", "builtin", "noglob":
		return true
	default:
		return false
	}
}

// has reports whether the call carries one of the flags as its own word.
func (c gitCall) has(flags ...string) bool {
	return slices.ContainsFunc(c.args, func(a string) bool { return slices.Contains(flags, a) })
}

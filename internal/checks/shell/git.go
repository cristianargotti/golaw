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
		if gitValueOption(args[0]) && len(args) > 1 {
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

// gitValueOption reports whether a global option of git takes the next
// word as its value when = does not join one, as git 2.44 reads them.
func gitValueOption(option string) bool {
	switch option {
	case "-C", "-c", "--git-dir", "--work-tree", "--namespace", "--config-env", "--attr-source", "--shallow-file":
		return true
	default:
		return false
	}
}

// unwrap drops the leading variable assignments, the reserved words a
// shell puts before the command it runs (reserved), and the wrappers that
// run their arguments as the command (wrapperOf), each past its own
// options, so a rule sees the program; a wrapper or a reserved word with
// nothing after it is the program itself.
func unwrap(seg segment) []string {
	words := []string(seg)
	for len(words) > 0 {
		w, wraps := wrapperOf(words[0])
		var command []string
		if wraps {
			command = w.command(words[1:])
		}
		switch {
		case assignment.MatchString(words[0]):
			words = words[1:]
		case len(command) > 0:
			words = command
		case reserved(words[0]) && len(words) > 1:
			words = words[1:]
		default:
			return words
		}
	}
	return words
}

// reserved reports whether a word is one a shell reads before the command
// it runs: the if, then, elif, else, while, until and do of a POSIX shell,
// its negation and its opening brace, and the try and finally that open a
// block in PowerShell, whose keywords take any case.
func reserved(word string) bool {
	switch strings.ToLower(word) {
	case "if", "then", "elif", "else", "while", "until", "do", "!", "{", "try", "finally":
		return true
	default:
		return false
	}
}

// has reports whether the call carries one of the flags as its own word.
func (c gitCall) has(flags ...string) bool {
	return slices.ContainsFunc(c.args, func(a string) bool { return slices.Contains(flags, a) })
}

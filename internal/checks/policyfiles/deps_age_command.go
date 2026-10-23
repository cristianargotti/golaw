package policyfiles

import (
	"slices"
	"strconv"
	"strings"
)

// The depsage invocations the check recognizes: the kit's tool by its path
// and the published tool at a literal pin; seven days is depsage's default.
const (
	depsAgeLocalPackage  = "./tools/depsage"
	depsAgeModulePackage = "github.com/cristianargotti/golaw/tools/depsage"
	depsAgeMinimum       = 7
	depsAgeDaysFlag      = "days"
	depsAgeAllowFlag     = "allow"
	depsAgeVersionParts  = 3
	depsAgeShellBytes    = "$`'\"\\|;&<>()*?[]{}"
)

// depsAgeCommand is a recipe line after its Make prefixes and literal assignments.
type depsAgeCommand struct {
	words   []string
	ignored bool
}

// dependencyAgeCommand reads one recipe line: the effective minimum age of
// a recognized depsage invocation and whether the line is one. A wrapper, a
// substitution, a computed word or an unknown executable is a stated limit,
// because a number found in their text proves nothing; an ignored failure
// has an effective age of zero, because the target no longer stops on it.
func dependencyAgeCommand(file string, line int, command string) (int, bool, error) {
	parsed, err := depsAgeParse(file, line, command)
	if err != nil || len(parsed.words) == 0 {
		return 0, false, err
	}
	if !depsAgeInvocation(parsed.words) {
		return 0, false, depsAgeOther(file, line, parsed.words)
	}
	days, err := depsAgeFlags(file, line, parsed.words[depsAgeVersionParts:])
	if err != nil || parsed.ignored {
		return 0, err == nil, err
	}
	return days, true, nil
}

// depsAgeParse joins a continued line, reads the Make prefixes and the
// literal environment assignments golaw.mk writes before a pinned tool, and
// refuses a shell operator, because a second command could carry the check.
func depsAgeParse(file string, line int, command string) (depsAgeCommand, error) {
	var parsed depsAgeCommand
	text := strings.TrimLeft(strings.ReplaceAll(command, "\\\n", " "), " \t")
	for text != "" && strings.ContainsRune("@-+", rune(text[0])) {
		parsed.ignored = parsed.ignored || text[0] == '-'
		text = strings.TrimLeft(text[1:], " \t")
	}
	if operator := depsAgeOperator(text); operator != "" {
		return parsed, outside(file, line, "the shell operator "+operator+" in the recipe")
	}
	words := strings.Fields(text)
	for len(words) != 0 {
		name, value, assignment := strings.Cut(words[0], "=")
		if !assignment || !depsAgeVariable(name) {
			break
		}
		if !depsAgeLiteral(value) {
			return parsed, outside(file, line, "the computed environment "+words[0])
		}
		words = words[1:]
	}
	parsed.words = words
	return parsed, nil
}

// depsAgeOperator answers the first shell operator of a line: a pipe, a
// separator, a redirection, a backquote or the $$ make hands the shell.
func depsAgeOperator(text string) string {
	operator, variable := strings.IndexAny(text, "|;&<>`"), strings.Index(text, "$$")
	switch {
	case operator >= 0 && (variable < 0 || operator < variable):
		return text[operator : operator+1]
	case variable >= 0:
		return "$$"
	}
	return ""
}

// depsAgeVariable answers whether a word names a shell variable, as the
// shell reads an assignment prefix.
func depsAgeVariable(name string) bool {
	rest := strings.TrimLeft(name, "_abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	return name != "" && rest == "" && (name[0] < '0' || name[0] > '9')
}

// depsAgeLiteral answers whether a word reaches the tool as written.
func depsAgeLiteral(word string) bool {
	return word != "" && !strings.ContainsAny(word, depsAgeShellBytes)
}

// depsAgeInvocation answers whether the words run depsage by its path or its pin.
func depsAgeInvocation(words []string) bool {
	if len(words) < depsAgeVersionParts || words[0] != "go" || words[1] != buildReaderRun {
		return false
	}
	version, pinned := strings.CutPrefix(words[2], depsAgeModulePackage+"@")
	return words[2] == depsAgeLocalPackage || pinned && depsAgePinned(version)
}

// depsAgePinned answers whether a version is a literal semantic version,
// with the optional literal pre-release a development build carries.
func depsAgePinned(version string) bool {
	core, rest, _ := strings.Cut(strings.TrimPrefix(version, "v"), "-")
	parts := strings.Split(core, ".")
	if !strings.HasPrefix(version, "v") || len(parts) != depsAgeVersionParts {
		return false
	}
	for _, part := range parts {
		if _, err := strconv.ParseUint(part, 10, 0); err != nil {
			return false
		}
	}
	return rest == "" || depsAgeLiteral(rest)
}

// depsAgeOther judges a line that runs something else: a command that cannot
// run the check is skipped; one that could run it is a stated limit, never a finding.
func depsAgeOther(file string, line int, words []string) error {
	switch {
	case slices.Contains([]string{"echo", "printf", "true", "git"}, words[0]):
		return nil
	case words[0] == "go" && len(words) > 1 && (words[1] == buildReaderRun || words[1] == "tool"):
		return outside(file, line, "go "+words[1]+" of another program, which could run the dependency age check")
	case words[0] == "go":
		return nil
	}
	return outside(file, line, words[0]+", an executable the check does not recognize, which could run the dependency age check")
}

// depsAgeFlags reads the arguments as the Go flag parser does, the last
// -days winning and -allow consumed; any other argument is a stated limit.
func depsAgeFlags(file string, line int, args []string) (int, error) {
	days := depsAgeMinimum
	for index := 0; index < len(args); {
		name, value, next, err := depsAgeFlag(file, line, args, index)
		if err != nil {
			return 0, err
		}
		index = next
		if name != depsAgeDaysFlag {
			continue
		}
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return 0, outside(file, line, "-days "+value+", which is not a number")
		}
		days = parsed
	}
	return days, nil
}

// depsAgeFlag reads one flag and its literal value and answers the index after it.
func depsAgeFlag(file string, line int, args []string, index int) (string, string, int, error) {
	word := args[index]
	name, value, inline := strings.Cut(strings.TrimPrefix(strings.TrimPrefix(word, "-"), "-"), "=")
	if !strings.HasPrefix(word, "-") || name != depsAgeDaysFlag && name != depsAgeAllowFlag {
		return "", "", 0, outside(file, line, word+", which is not an argument of depsage the check reads")
	}
	next := index + 1
	if !inline {
		if next == len(args) {
			return "", "", 0, outside(file, line, "-"+name+" without a value")
		}
		value, next = args[next], next+1
	}
	if !depsAgeLiteral(value) {
		return "", "", 0, outside(file, line, "the computed value "+value+" of -"+name)
	}
	return name, value, next, nil
}

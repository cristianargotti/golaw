package shell

import (
	"regexp"
	"slices"
	"strings"
)

// writeOperator is a redirection that writes its file: >, >>, >| and the
// same with a stream before them (2>, &>, *> in PowerShell). A duplication
// such as 2>&1 and every input operator are left out.
var writeOperator = regexp.MustCompile(`^(?:\d+|&|\*)?>>?\|?$`)

// The long options of sed that give its script, compared with and
// without their value.
const (
	expressionOption = "--expression"
	fileOption       = "--file"
)

// sedValued are the options of sed that take the next word as their value,
// so the value is never read as a file.
func sedValued() map[string]bool {
	return map[string]bool{"-e": true, "-f": true, "-l": true, expressionOption: true, fileOption: true, "--line-length": true}
}

// copyValued are the options of cp and mv that take the next word.
func copyValued() map[string]bool {
	return map[string]bool{"-S": true, "--suffix": true, "-t": true, "--target-directory": true}
}

// WriteTargets answers the files a command writes, moves or removes that
// its words name literally: the file of a write redirection and the
// operands of tee, rm, cp, mv and sed in place, sorted and distinct. A
// word the shell would expand (a variable, a substitution, a glob, a home
// path) is left out, because a guard denies only what it reads with
// certainty (M7.9.2).
func WriteTargets(command, tool string) []string {
	var out []string
	for _, seg := range segments(command, tool) {
		for _, word := range segmentTargets(seg) {
			if literal(word) {
				out = append(out, word)
			}
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// segmentTargets answers the target words of one segment: the file of a
// redirection, or the operands its program writes.
func segmentTargets(seg segment) []string {
	if len(seg) == 2 && writeOperator.MatchString(seg[0]) {
		return []string{seg[1]}
	}
	words := unwrap(seg)
	if len(words) == 0 {
		return nil
	}
	args := words[1:]
	switch base(words[0]) {
	case "tee", "rm":
		ops, _ := operands(args, nil)
		return ops
	case "cp":
		return copyTargets(args, false)
	case "mv":
		return copyTargets(args, true)
	case "sed":
		return sedTargets(args)
	default:
		return nil
	}
}

// copyTargets answers the destination of a copy or a move, and every
// source of a move, which it leaves; a target directory option names a
// directory, never the files, so the command is left to the post feedback.
func copyTargets(args []string, move bool) []string {
	ops, opts := operands(args, copyValued())
	for _, o := range opts {
		if o == "--target-directory" || strings.HasPrefix(o, "--target-directory=") || strings.HasPrefix(o, "-t") {
			return nil
		}
	}
	if len(ops) < 2 {
		return nil
	}
	if move {
		return ops
	}
	return ops[len(ops)-1:]
}

// sedTargets answers the files sed edits in place; without -i it writes
// standard output only. The first operand is the script unless an option
// gave it.
func sedTargets(args []string) []string {
	ops, opts := operands(args, sedValued())
	inPlace, scripted := false, false
	for _, o := range opts {
		switch {
		case o == "--in-place" || strings.HasPrefix(o, "--in-place="), strings.HasPrefix(o, "-i"):
			inPlace = true
		case o == expressionOption || o == fileOption || strings.HasPrefix(o, expressionOption+"=") || strings.HasPrefix(o, fileOption+"="),
			strings.HasPrefix(o, "-e") || strings.HasPrefix(o, "-f"):
			scripted = true
		}
	}
	if !inPlace {
		return nil
	}
	if !scripted && len(ops) > 0 {
		ops = ops[1:]
	}
	return ops
}

// operands splits the arguments of a program into its operands and its
// options: an option in valued takes the next word, and -- ends the
// options.
func operands(words []string, valued map[string]bool) ([]string, []string) {
	var ops, opts []string
	for i := 0; i < len(words); i++ {
		w := words[i]
		switch {
		case w == "--":
			return append(ops, words[i+1:]...), opts
		case len(w) > 1 && strings.HasPrefix(w, "-"):
			opts = append(opts, w)
			if valued[w] {
				i++
			}
		default:
			ops = append(ops, w)
		}
	}
	return ops, opts
}

// literal reports whether a word names one file as written: no expansion,
// no pattern and no home path, which only the shell would resolve.
func literal(word string) bool {
	return word != "" && word != "-" && !strings.ContainsAny(word, "$`*?[{") && !strings.HasPrefix(word, "~")
}

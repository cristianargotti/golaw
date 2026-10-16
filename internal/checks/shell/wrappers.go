package shell

import (
	"slices"
	"strings"
)

// wrapper is how a program that runs its arguments as the command reads
// its own options before that command: the short options that take a
// value, attached or in the next word, the ones that take an attached
// value only, the long ones that take a value unless = joins it, the
// positional words before the command, and the options whose value is a
// command line the program splits into the words the command starts with.
// Every list of names is spaced at both ends.
type wrapper struct {
	short, optional, long string
	positional            int
	split                 string
}

// wrapperOf answers the options of a program that runs its arguments as
// the command, GNU and BSD together, and false for any other program.
func wrapperOf(name string) (wrapper, bool) {
	switch base(name) {
	case "env":
		return wrapper{short: "uCSP", long: " unset chdir split-string ", split: " S split-string "}, true
	case "sudo":
		return wrapper{short: "ughprtTUCDR", long: " user group host prompt role type command-timeout other-user close-from chdir chroot "}, true
	case "doas":
		return wrapper{short: "uC"}, true
	case "nice":
		return wrapper{short: "n", long: " adjustment "}, true
	case "time":
		return wrapper{short: "of", long: " output format "}, true
	case "exec":
		return wrapper{short: "a"}, true
	case "timeout":
		return wrapper{short: "sk", long: " signal kill-after ", positional: 1}, true
	case "xargs":
		return wrapper{short: "adEIJLnPRSs", optional: "eil", long: " arg-file delimiter max-args max-procs max-chars process-slot-var "}, true
	case "command", "nohup", "builtin", "noglob":
		return wrapper{}, true
	default:
		return wrapper{}, false
	}
}

// command reads the options of the wrapper from the index from and
// answers the words, the string of a split option replaced by the words it
// holds where it stood, which env reads on as options and command, and the
// index of the command after the options and the positional words: an
// option is a word that starts with a dash, and -- ends them.
func (w wrapper) command(words []string, from int) ([]string, int) {
	i := from
	for i < len(words) && strings.HasPrefix(words[i], "-") {
		word, at := words[i], i
		i++
		if word == "--" {
			break
		}
		name, value, next := w.option(word)
		if next && i < len(words) {
			value = words[i]
			i++
		}
		if name != "" && strings.Contains(w.split, " "+name+" ") {
			words, i = splitString(words, at, i, name, value)
		}
	}
	return words, i + w.positional
}

// splitString answers the words with the split option at the index at and
// its string, through the index end, replaced by the words of the string,
// the switches a cluster holds before the option kept, and the index of
// the first word after those switches, where reading goes on.
func splitString(words []string, at, end int, name, value string) ([]string, int) {
	var head []string
	if word := words[at]; !strings.HasPrefix(word, "--") {
		if cluster, _, found := strings.Cut(word, name); found && len(cluster) > 1 {
			head = []string{cluster}
		}
	}
	return slices.Concat(words[:at], head, fields(value), words[end:]), at + len(head)
}

// option reads an option word and answers the name of the option that
// takes a value, that value when it is attached, and whether it is the
// next word instead: a long option of the list takes the value = joins or
// the next word, and a cluster of short options ends at the first one
// that takes a value, which takes the rest of the cluster or, when it
// ends the cluster, the next word.
func (w wrapper) option(word string) (string, string, bool) {
	if long, ok := strings.CutPrefix(word, "--"); ok {
		name, value, joined := strings.Cut(long, "=")
		if !strings.Contains(w.long, " "+name+" ") {
			return "", "", false
		}
		return name, value, !joined
	}
	for i := 1; i < len(word); i++ {
		switch letter := word[i : i+1]; {
		case strings.Contains(w.short, letter):
			return letter, word[i+1:], i+1 == len(word)
		case strings.Contains(w.optional, letter):
			return "", "", false
		}
	}
	return "", "", false
}

// fields splits a command line into words by the quotes of a POSIX shell,
// the way env -S splits its string.
func fields(line string) []string {
	var out []string
	for _, seg := range scan(line, posix).out {
		out = append(out, seg...)
	}
	return out
}

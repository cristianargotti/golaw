package policyfiles

import (
	"path"
	"regexp"
	"slices"
	"strings"
)

// installWord is a package an install line may name: the name apk and apt
// accept, then an optional =version; anything else in a package list (a
// variable, a substitution, a quote, a glob, a release suffix) is outside
// the declared subset.
var installWord = regexp.MustCompile(`^([a-z0-9][a-z0-9+._-]*)(?:=([A-Za-z0-9+.~:_-]+))?$`)

// installFlag reports an option of apk add, apt-get install or apt install
// that takes no argument, so the word after it is still a package; any
// other option is outside the declared subset, because its argument would
// read as a package.
func installFlag(word string) bool {
	switch word {
	case "--no-cache", "--update", "-U", "--upgrade", "-u", "--quiet", "-q", "-qq", "--no-progress",
		"-y", "--yes", "--assume-yes", "--no-install-recommends", "--no-install-suggests":
		return true
	default:
		return false
	}
}

// installed is one package an install line names, at the line of its RUN
// instruction.
type installed struct {
	name, version string
	line          int
}

// dockerComment is a comment line of the Dockerfile, outside heredoc bodies.
type dockerComment struct {
	text string
	line int
}

// imageCommands is what the install rules read from a Dockerfile: every
// package the RUN instructions install and every comment line.
type imageCommands struct {
	path     string
	installs []installed
	comments []dockerComment
}

// readInstalls walks a Dockerfile the way readDockerfile does (logical
// lines joined as BuildKit joins them, heredoc bodies skipped) and reads
// the RUN instructions in shell form: each command of a && or ; chain that
// is apk add, apt-get install or apt install names its packages.
func readInstalls(file string, data []byte) (imageCommands, error) {
	doc := imageCommands{path: file}
	lines := strings.Split(strings.TrimPrefix(string(data), "\ufeff"), "\n")
	for index := range lines {
		lines[index] = strings.TrimSuffix(lines[index], "\r")
	}
	index := 0
	for index < len(lines) {
		if err := doc.parseLine(lines, &index); err != nil {
			return imageCommands{}, err
		}
		index++
	}
	return doc, nil
}

func (d *imageCommands) parseLine(lines []string, index *int) error {
	text, line := strings.TrimSpace(lines[*index]), *index+1
	switch {
	case text == "":
		return nil
	case strings.HasPrefix(text, "#"):
		d.comments = append(d.comments, dockerComment{text: text, line: line})
		return nil
	}
	text, complete := dockerLogicalLine(lines, index)
	if !complete {
		return outside(d.path, line, "unfinished backslash continuation")
	}
	start := *index
	if !dockerBodies(lines, index, text) {
		return outside(d.path, line, "unsupported or unterminated heredoc")
	}
	command, rest := dockerCommand(text)
	if command != dockerRun {
		return nil
	}
	if names := strings.Join(lines[start+1:*index+1], "\n"); *index > start && installsIn(names) {
		return outside(d.path, line, "a package installation inside a RUN heredoc")
	}
	return d.run(rest, line)
}

// run reads the commands of one RUN instruction after its flags.
func (d *imageCommands) run(rest string, line int) error {
	rest = strings.TrimSpace(rest)
	if strings.HasPrefix(rest, "[") {
		if installsIn(rest) {
			return outside(d.path, line, "a package installation in the exec form of RUN")
		}
		return nil
	}
	for strings.HasPrefix(rest, "--") {
		_, tail, _ := strings.Cut(rest, " ")
		rest = strings.TrimSpace(tail)
	}
	for _, segment := range strings.FieldsFunc(strings.ReplaceAll(rest, "&&", ";"), func(r rune) bool { return r == ';' }) {
		if err := d.command(strings.Fields(segment), line); err != nil {
			return err
		}
	}
	return nil
}

// command records the packages of one install command; a command that
// installs nothing is not read, and an install in a form installArguments
// does not read is a stated limit, never a clean answer.
func (d *imageCommands) command(words []string, line int) error {
	packages, ok := installArguments(words)
	if !ok {
		if unsplitInstall(words) {
			return outside(d.path, line, "install command in a form the reader does not split: "+strings.Join(words, " "))
		}
		return nil
	}
	for _, word := range packages {
		switch {
		case installFlag(word):
			continue
		case strings.HasPrefix(word, "-"):
			return outside(d.path, line, "install option "+word)
		}
		match := installWord.FindStringSubmatch(word)
		if match == nil {
			return outside(d.path, line, "package word "+word)
		}
		d.installs = append(d.installs, installed{name: match[1], version: match[2], line: line})
	}
	return nil
}

// installArguments answers the words after the install verb of apk add,
// apt-get install or apt install, options before the verb skipped.
func installArguments(words []string) ([]string, bool) {
	if len(words) == 0 {
		return nil, false
	}
	verb := installVerb(words[0])
	for i := 1; verb != "" && i < len(words); i++ {
		if !strings.HasPrefix(words[i], "-") {
			return words[i+1:], words[i] == verb
		}
	}
	return nil, false
}

// installVerb is the install verb of a package manager the reader knows,
// empty for any other word.
func installVerb(manager string) string {
	switch manager {
	case "apk":
		return "add"
	case "apt-get", "apt":
		return "install"
	}
	return ""
}

// unsplitInstall reports whether words name a package manager, by its base
// name and outside a parenthesis, followed by its install verb: the install
// behind an assignment, a shell keyword, a subshell, sudo, a path or an
// option that takes an argument, which installArguments does not read. A
// manager without its install verb after it, such as apt-get update,
// installs nothing.
func unsplitInstall(words []string) bool {
	for i, word := range words {
		verb := installVerb(path.Base(strings.Trim(word, "()")))
		if verb != "" && slices.Contains(words[i+1:], verb) {
			return true
		}
	}
	return false
}

// installsIn reports whether a text names a package manager's install
// command, for the forms the reader does not split.
func installsIn(text string) bool {
	for _, verb := range []string{"apk", "apt-get", "apt"} {
		if slices.ContainsFunc(strings.FieldsFunc(text, func(r rune) bool { return strings.ContainsRune(" \t\n\",[]", r) }), func(word string) bool { return word == verb }) {
			return true
		}
	}
	return false
}

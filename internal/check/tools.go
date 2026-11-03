package check

import (
	"regexp"
	"strconv"
	"strings"
)

// locatedPattern matches a line that names a Go file and a line of it, the
// shape go vet, the compiler and golangci-lint report a defect in.
var locatedPattern = regexp.MustCompile(`^\S+\.go:\d+`)

// Located answers the lines of a tool's output that name a file and a
// line, with the ./ and vet: prefixes go vet adds removed, so every gate
// prints a failure the same way.
func Located(text string) []string {
	var out []string
	for _, line := range nonBlank(text) {
		line = strings.TrimPrefix(strings.TrimPrefix(line, "vet: "), "./")
		if locatedPattern.MatchString(line) {
			out = append(out, clip(line))
		}
	}
	return out
}

// Unformatted names each file of a golangci-lint fmt --diff, whose +++
// lines carry the formatted side of each file.
func Unformatted(diff string) []string {
	var out []string
	for line := range strings.SplitSeq(diff, "\n") {
		name, ok := strings.CutPrefix(line, "+++ ")
		if !ok {
			continue
		}
		name, _, _ = strings.Cut(name, "\t")
		name = strings.TrimPrefix(strings.TrimPrefix(name, "b/"), "./")
		out = append(out, name+": not formatted, run golangci-lint fmt")
	}
	return out
}

// Failed answers the lines of a tool that exited with an error: the
// located lines when it printed some, else its first lines, else the
// exit status, so a red gate always says something.
func Failed(tool string, code int, stdout, stderr []byte) []string {
	text := string(stdout) + "\n" + string(stderr)
	if located := Located(text); len(located) > 0 {
		return located
	}
	lines := nonBlank(text)
	if len(lines) == 0 {
		return []string{tool + " exited " + strconv.Itoa(code)}
	}
	for i := range lines {
		lines[i] = clip(lines[i])
	}
	return lines
}

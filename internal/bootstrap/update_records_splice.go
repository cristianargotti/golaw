package bootstrap

import (
	"slices"
	"strings"
)

// frontDelimiter opens and closes the front matter of a decision record.
const frontDelimiter = "---"

// withModules adds module entries to the modules key of the front matter,
// in the shapes init writes: an empty flow list on its own line or after the
// key, or a block list, whose last entry the new ones follow. The result is
// decoded again, so a splice that does not record every module is no
// proposal.
func withModules(before []byte, modules []string) ([]byte, bool) {
	newline := policyNewline(before)
	lines := strings.SplitAfter(string(before), "\n")
	index := modulesKey(lines)
	if index < 0 {
		return nil, false
	}
	items := make([]string, 0, len(modules))
	for _, module := range modules {
		items = append(items, "  - "+module+newline)
	}
	end := index + 1
	for end < len(lines) && strings.HasPrefix(lines[end], "  - ") {
		end++
	}
	switch {
	case strings.TrimRight(lines[index], "\r\n") == "modules: []":
		lines = slices.Concat(lines[:index], []string{"modules:" + newline}, items, lines[index+1:])
	case end == index+1 && end < len(lines) && strings.TrimSpace(lines[end]) == "[]":
		lines = slices.Concat(lines[:end], items, lines[end+1:])
	case end > index+1:
		lines = slices.Concat(lines[:end], items, lines[end:])
	default:
		return nil, false
	}
	after := []byte(strings.Join(lines, ""))
	decoded, ok := frontModules(after)
	return after, ok && !slices.ContainsFunc(modules, func(m string) bool { return !slices.Contains(decoded, m) })
}

// modulesKey answers the line of the modules key inside the front matter
// that opens the record, or -1.
func modulesKey(lines []string) int {
	if strings.TrimRight(lines[0], "\r\n") != frontDelimiter {
		return -1
	}
	for index := 1; index < len(lines); index++ {
		switch strings.TrimRight(lines[index], "\r\n") {
		case frontDelimiter:
			return -1
		case "modules:", "modules: []":
			return index
		}
	}
	return -1
}

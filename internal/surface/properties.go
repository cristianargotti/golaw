package surface

import (
	"maps"
	"slices"
	"strings"
)

// The keys of sonar-project.properties the scope reads (M6.3.2); every
// other key is ignored.
const (
	keySources            = "sonar.sources"
	keyTests              = "sonar.tests"
	keyInclusions         = "sonar.inclusions"
	keyExclusions         = "sonar.exclusions"
	keyTestInclusions     = "sonar.test.inclusions"
	keyTestExclusions     = "sonar.test.exclusions"
	keyCoverageExclusions = "sonar.coverage.exclusions"
	keyCPDExclusions      = "sonar.cpd.exclusions"
	keyCoverageReports    = "sonar.go.coverage.reportPaths"
)

// The causes of a read key golaw does not resolve.
const (
	causePlaceholder = "a ${...} placeholder golaw does not resolve"
	causeEscape      = `a \u escape golaw does not decode`
)

// Properties is what the declared subset of the Java properties format
// read from sonar-project.properties.
type Properties struct {
	Values     map[string][]string // the read keys of M6.3.2, each a comma separated list split and trimmed
	Unmeasured []string            // "key: reason" for a ${...} placeholder or a \u escape in a read key
}

// readKey reports the keys of M6.3.2.
func readKey(key string) bool {
	switch key {
	case keySources, keyTests, keyInclusions, keyExclusions, keyTestInclusions, keyTestExclusions,
		keyCoverageExclusions, keyCPDExclusions, keyCoverageReports:
		return true
	}
	return false
}

// ParseProperties reads the subset of M6.3.2: key=value, key: value and
// key value, # and ! comment lines, a trailing backslash continuation, the
// escapes \t \n \r \f and a backslash before any other character; the last
// definition of a key wins, as Java's loader decides.
func ParseProperties(data []byte) Properties {
	out := Properties{Values: map[string][]string{}}
	unmeasured := map[string]string{}
	for _, line := range logicalLines(string(data)) {
		rawKey, rawValue := splitEntry(line)
		key := unescape(rawKey)
		switch {
		case strings.Contains(rawKey, `\u`) && strings.HasPrefix(rawKey, "sonar."):
			unmeasured[rawKey] = causeEscape
		case !readKey(key):
		case strings.Contains(rawValue, `\u`):
			delete(out.Values, key)
			unmeasured[key] = causeEscape
		case strings.Contains(rawValue, "${"):
			delete(out.Values, key)
			unmeasured[key] = causePlaceholder
		default:
			delete(unmeasured, key)
			out.Values[key] = splitList(unescape(rawValue))
		}
	}
	for _, key := range slices.Sorted(maps.Keys(unmeasured)) {
		out.Unmeasured = append(out.Unmeasured, key+": "+unmeasured[key])
	}
	return out
}

// logicalLines joins the continuations and drops the blank and comment
// lines; a continuation line keeps its content even when it starts with #.
func logicalLines(text string) []string {
	var out []string
	current, continuing := "", false
	for raw := range strings.SplitSeq(text, "\n") {
		line := strings.TrimLeft(strings.TrimSuffix(raw, "\r"), " \t\f")
		if !continuing && (line == "" || line[0] == '#' || line[0] == '!') {
			continue
		}
		body, more := continued(line)
		current += body
		if continuing = more; !more {
			out = append(out, current)
			current = ""
		}
	}
	if current != "" {
		out = append(out, current)
	}
	return out
}

// continued reports a line that ends in an odd run of backslashes, and the
// line without the one that continues it.
func continued(line string) (string, bool) {
	trailing := len(line) - len(strings.TrimRight(line, `\`))
	if trailing%2 == 1 {
		return line[:len(line)-1], true
	}
	return line, false
}

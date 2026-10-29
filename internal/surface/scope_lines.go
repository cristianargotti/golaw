package surface

import (
	"strconv"
	"strings"
)

// The key prefix and the source word of the scope lines config explain
// prints after the policy's own lines.
const (
	linePrefix = "surface.scope."
	computed   = "computed"
)

// Lines answers the scope in force as config explain prints it,
// "surface.scope.<key> = <value> (<origin>)": the source and the origin,
// every list that holds an entry, the coverage report and profile, then
// one line per cause left Unmeasured. A scope read from no file names
// computed as its origin.
func (s Scope) Lines() []string {
	origin, value := s.Origin, s.Origin
	if origin == "" {
		origin, value = computed, SourceNone
	}
	out := []string{line("source", s.Source, origin), line("origin", value, origin)}
	lists := []namedList{{"sources", &s.Sources}, {"tests", &s.Tests}}
	for _, list := range append(lists, patternLists(&s)...) {
		if len(*list.entries) > 0 {
			out = append(out, line(list.name, "["+strings.Join(*list.entries, ", ")+"]", origin))
		}
	}
	out = append(out,
		line("coverage_report", s.CoverageReport, origin),
		line("coverage_profile", "coverpkg "+strconv.FormatBool(s.Coverage.CoverPkg)+", covermode "+s.Coverage.CoverMode, origin))
	for _, cause := range s.Unmeasured {
		out = append(out, line("unmeasured", cause, origin))
	}
	return out
}

// line renders one scope line.
func line(key, value, origin string) string {
	return linePrefix + key + " = " + value + " (" + origin + ")"
}

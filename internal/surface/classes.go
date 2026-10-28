package surface

import (
	"bytes"
	"path"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Class says whether the native checks and the linters judge a surface file
// (M6.3.1): only ClassSource is judged whole; a declaration file is judged
// by name rules only.
type Class int

// The surface classes.
const (
	ClassSource      Class = iota + 1 // hand written source the checks judge
	ClassMinified                     // a .min. segment, or an average line of at least 200 characters
	ClassGenerated                    // a generated header in any comment syntax, or a path under gen/, sqlcgen/, test/mocks/
	ClassFixture                      // a path with a testdata segment
	ClassDeclaration                  // a .d.ts file, judged by name rules only
)

// minifiedAverage is the average line length, in characters, from which a
// script or a style sheet is minified: Sonar's JS analyzer skips those.
const minifiedAverage = 200

// generatedMarker is the Go convention of a generated header in the line
// comment of any language a surface file is written in; requiring the
// comment at the start of a line keeps a string that quotes it out.
var generatedMarker = regexp.MustCompile(`(?m)^[ \t]*(?://|#|/\*|<!--|--)[ \t]*(?:Package \S+ )?Code generated .* DO NOT EDIT\.?`)

// String names the class for a report line.
func (c Class) String() string {
	switch c {
	case ClassSource:
		return "source"
	case ClassMinified:
		return "minified"
	case ClassGenerated:
		return "generated"
	case ClassFixture:
		return "fixture"
	case ClassDeclaration:
		return "declaration"
	}
	return "Class(" + strconv.Itoa(int(c)) + ")"
}

// classOf answers the class of a surface file, the first that holds in the
// order generated, fixture, minified, declaration; nil content judges the
// name alone.
func classOf(name string, language Language, content []byte) Class {
	base := path.Base(name)
	switch {
	case generatedPath(name) || generatedMarker.Match(content):
		return ClassGenerated
	case name == "testdata" || strings.HasPrefix(name, "testdata/") || strings.Contains(name, "/testdata/"):
		return ClassFixture
	case strings.Contains(base, ".min.") || longLines(language, content):
		return ClassMinified
	case strings.HasSuffix(base, ".d.ts"):
		return ClassDeclaration
	}
	return ClassSource
}

// generatedPath reports the generated directories of the pipeline's
// exclusions: a gen or sqlcgen segment anywhere, test/mocks at the root.
func generatedPath(name string) bool {
	for segment := range strings.SplitSeq(path.Dir(name), "/") {
		if segment == "gen" || segment == "sqlcgen" {
			return true
		}
	}
	return strings.HasPrefix(name, "test/mocks/")
}

// longLines reports a script or a style sheet whose average line, line
// breaks left out, holds at least minifiedAverage characters. Data files
// keep their class: a one line template is still judged.
func longLines(language Language, content []byte) bool {
	if !scriptOrStyle(language) {
		return false
	}
	breaks := bytes.Count(content, []byte("\n"))
	lines := breaks
	if len(content) > 0 && content[len(content)-1] != '\n' {
		lines++
	}
	if lines == 0 {
		return false
	}
	return (utf8.RuneCount(content)-breaks)/lines >= minifiedAverage
}

// scriptOrStyle reports the languages Sonar skips when minified.
func scriptOrStyle(language Language) bool {
	return language == JavaScript || language == TypeScript || language == JSX || language == TSX || language == CSS
}

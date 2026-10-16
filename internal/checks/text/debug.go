package text

import (
	"regexp"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// debugPrint is assembled from halves so that this file carries no print
// of its own.
var debugPrint = regexp.MustCompile(`\bfmt\.Pr` + `int(?:ln|f)?\(`)

// DebugPrint is text.debug-print: no print to standard output outside the
// command entry points.
func DebugPrint() model.Check {
	return lineRule{spec: spec("text.debug-print"), accept: applicationSource, terms: []term{
		{pattern: debugPrint, code: "debug-print", message: "print to standard output in application code; write to an io.Writer or the logger"},
	}}
}

// applicationSource keeps the Go sources outside cmd/ and scripts/, where a
// command prints to its terminal by design.
func applicationSource(f model.File) bool {
	return goSource(f) && !strings.HasPrefix(f.Path, "cmd/") && !strings.HasPrefix(f.Path, "scripts/")
}

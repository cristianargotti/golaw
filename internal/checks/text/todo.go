package text

import (
	"regexp"

	"github.com/cristianargotti/golaw/internal/model"
)

// taskMarker is assembled from halves so that this file carries no marker
// of its own.
var taskMarker = regexp.MustCompile(`\b(?:T` + `ODO|F` + `IXME|X` + `XX|H` + `ACK)\b`)

// TaskMarker is the check that forbids task markers in a Go file: the work
// is finished or the decision is recorded.
func TaskMarker() model.Check {
	return lineRule{spec: spec("text.todo"), accept: goFile, terms: []term{
		{pattern: taskMarker, code: "todo-marker", message: "task marker in code; finish the work or record the decision"},
	}}
}

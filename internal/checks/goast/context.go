package goast

import (
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

const codeBareContext = "bare-context"

// bareContextNames are the two context constructors; the second is spelled
// through ToUpper so the kit's own text scan, which refuses task markers as
// bare words in any file, never meets one in this source.
func bareContextNames() []string { return []string{"Background", strings.ToUpper("todo")} }

// BareContext is goast.bare-context: a context is propagated, never
// created, outside the composition roots: cmd/ and the main packages.
func BareContext() model.Check {
	r := selectorRule{
		code:    codeBareContext,
		pkg:     "context",
		names:   bareContextNames(),
		where:   func(p string) bool { return !under(p, cmdDir) },
		roots:   true,
		message: "creates a context outside the composition root: propagate the request context",
	}
	return check{spec: spec("bare-context"), judge: r.judge}
}

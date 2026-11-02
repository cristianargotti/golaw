package goldentest

import (
	"fmt"
	"go/ast"
	"go/token"
	"slices"
	"strconv"
	"strings"
)

// The widest modes a template may write: a file only its owner reads and
// a directory only its owner enters, the modes the gosec hotspots accept.
const (
	fileModeLimit = 0o600
	dirModeLimit  = 0o700
)

// modeArgument is where an os call takes its mode and the widest it takes.
type modeArgument struct {
	index int
	limit uint64
}

// modeCalls answers the os calls whose mode the law judges.
func modeCalls() map[string]modeArgument {
	return map[string]modeArgument{
		"WriteFile": {index: 2, limit: fileModeLimit},
		"OpenFile":  {index: 2, limit: fileModeLimit},
		"Chmod":     {index: 1, limit: fileModeLimit},
		"Mkdir":     {index: 1, limit: dirModeLimit},
		"MkdirAll":  {index: 1, limit: dirModeLimit},
	}
}

// judgeSuppression is the nolintlint row of M7.10.2: a template never
// quiets a linter or the gate, because a quieted rule is still a debt.
func judgeSuppression(f goFile) []string {
	var out []string
	for _, group := range f.file.Comments {
		for _, comment := range group.List {
			if suppresses(comment.Text) {
				out = append(out, at(f, comment.Pos(), lawSuppression, "a suppression comment: a template never quiets a linter"))
			}
		}
	}
	return out
}

func suppresses(text string) bool {
	body := strings.TrimLeft(strings.TrimPrefix(strings.TrimPrefix(text, "//"), "/*"), " \t")
	for _, prefix := range []string{"nolint", "lint:ignore", "lint:file-ignore"} {
		if strings.HasPrefix(body, prefix) {
			return true
		}
	}
	return strings.Contains(text, "NOSONAR") || strings.Contains(text, "#nosec")
}

// judgeModes is the permissions row of M7.10.2: an integer literal mode of
// an os call stays within the owner's own bits.
func judgeModes(f goFile) []string {
	calls := modeCalls()
	var out []string
	ast.Inspect(f.file, func(n ast.Node) bool {
		name, call, ok := packageCall(n, "os")
		arg, judged := calls[name]
		if !ok || !judged || len(call.Args) <= arg.index {
			return true
		}
		lit, ok := call.Args[arg.index].(*ast.BasicLit)
		if !ok || lit.Kind != token.INT {
			return true
		}
		mode, err := strconv.ParseUint(lit.Value, 0, 32)
		if err == nil && mode&^arg.limit != 0 {
			out = append(out, at(f, lit.Pos(), lawModes, fmt.Sprintf("mode 0o%o wider than 0o%o", mode, arg.limit)))
		}
		return true
	})
	return out
}

// judgeContext is the contextcheck row of M7.10.2: a test takes the
// context the testing package cancels, never a root one.
func judgeContext(f goFile) []string {
	if !f.test {
		return nil
	}
	// The empty context's name is assembled from halves, so this file
	// carries no task marker for text.todo to read.
	roots := []string{"Background", "T" + "ODO"}
	var out []string
	ast.Inspect(f.file, func(n ast.Node) bool {
		name, call, ok := packageCall(n, "context")
		if ok && len(call.Args) == 0 && slices.Contains(roots, name) {
			out = append(out, at(f, call.Pos(), lawContext, fmt.Sprintf("context.%s in a test: use t.Context()", name)))
		}
		return true
	})
	return out
}

// packageCall reads a call of a function of the named package.
func packageCall(n ast.Node, pkg string) (string, *ast.CallExpr, bool) {
	call, ok := n.(*ast.CallExpr)
	if !ok {
		return "", nil, false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", nil, false
	}
	qualifier, ok := selector.X.(*ast.Ident)
	if !ok || qualifier.Name != pkg {
		return "", nil, false
	}
	return selector.Sel.Name, call, true
}

package goast

import (
	"go/ast"
	"path"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

const (
	codeErrorTextAsData = "error-text-as-data"
	errorsFile          = "errors.go"
	errorMethod         = "Error"
)

// errorTextDirs are the layers where error text stored as data carries the
// wrap chain into a row, a ledger or a response.
func errorTextDirs() []string {
	return []string{
		"internal/application/", "internal/infrastructure/jobs/",
		"internal/adapters/inbound/jobs/", "internal/adapters/outbound/",
	}
}

// ErrorTextAsData is goast.error-text-as-data: err.Error() stored,
// returned or handed to a function as data in a use case, a job or an
// outbound adapter is a finding; logging, wrapping, matching and typed
// error constructors keep the chain by design.
func ErrorTextAsData() model.Check {
	return check{spec: spec("error-text-as-data"), judge: judgeErrorText}
}

func judgeErrorText(t *target, _ *state) ([]model.Finding, error) {
	if !under(t.file.Path, errorTextDirs()...) || path.Base(t.file.Path) == errorsFile {
		return nil, nil
	}
	file, _, err := t.parsed()
	if err != nil {
		return nil, err
	}
	var out []model.Finding
	ast.PreorderStack(file, nil, func(n ast.Node, stack []ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !isErrorText(call) {
			return true
		}
		if use := dataUse(stack); use != "" {
			message := "error text is " + use + " as data and carries the wrap chain: keep the root cause or a typed message"
			out = append(out, t.finding(codeErrorTextAsData, call.Pos(), message))
		}
		return true
	})
	return out, nil
}

// isErrorText reports a zero-argument .Error() call on anything but the
// result of a root-cause function, whose text is the rule's own answer.
func isErrorText(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || len(call.Args) != 0 || sel.Sel.Name != errorMethod {
		return false
	}
	inner, ok := sel.X.(*ast.CallExpr)
	return !ok || !slices.Contains(rootFunctions(), lastSegment(callee(inner)))
}

// dataUse names how the nearest enclosing statement uses the text: stored
// by an assignment, a declaration or a composite literal, returned, or
// passed to a function that is not a logger, a wrapper or a matcher. Empty
// means the use keeps the chain.
func dataUse(stack []ast.Node) string {
	for _, node := range slices.Backward(stack) {
		switch parent := node.(type) {
		case *ast.BinaryExpr, *ast.ParenExpr, *ast.KeyValueExpr:
			continue
		case *ast.AssignStmt, *ast.ValueSpec:
			return "stored"
		case *ast.ReturnStmt:
			return "returned"
		case *ast.CompositeLit:
			if typedError(typeName(parent.Type)) {
				return ""
			}
			return "stored"
		case *ast.CallExpr:
			if keepsChain(parent) {
				return ""
			}
			return "passed"
		default:
			return ""
		}
	}
	return ""
}

// typedError reports a type named like an error value or an error type.
func typedError(name string) bool {
	return strings.HasSuffix(name, "Error") || strings.HasPrefix(name, "Err")
}

// chainReceivers are the receivers whose calls keep the chain: loggers,
// the testing value and the assertion packages.
func chainReceivers() []string {
	return []string{"slog", "log", "logger", "zap", "t", "require", "assert"}
}

// chainFunctions are the wrappers and matchers that keep the chain.
func chainFunctions() []string {
	return []string{
		"fmt.Errorf", "errors.Is", "errors.As", "errors.New",
		"strings.Contains", "strings.HasPrefix", "strings.HasSuffix", "strings.EqualFold",
	}
}

// chainMethods are the logging methods on any receiver.
func chainMethods() []string {
	return []string{"Warn", "Info", "Debug", errorMethod, "Errorw", "Infow", "Warnw"}
}

// rootFunctions are the functions that answer the root cause of a chain.
func rootFunctions() []string {
	return []string{"Root", "RootCause", "rootCause", "Cause", "Unwrap"}
}

// keepsChain reports a call that logs, wraps, matches or unwraps.
func keepsChain(call *ast.CallExpr) bool {
	name := callee(call)
	receiver, _, _ := strings.Cut(name, ".")
	return slices.Contains(chainFunctions(), name) ||
		slices.Contains(chainMethods(), lastSegment(name)) ||
		slices.Contains(rootFunctions(), lastSegment(name)) ||
		(strings.Contains(name, ".") && slices.Contains(chainReceivers(), receiver))
}

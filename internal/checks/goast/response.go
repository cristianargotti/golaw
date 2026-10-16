package goast

import (
	"go/ast"
	"go/token"
	"path"
	"slices"
	"strconv"

	"github.com/cristianargotti/golaw/internal/model"
)

const (
	codeRawErrorResponse = "raw-error-response"
	errorsPackage        = "errors"
	contractMarker       = "contract:"
	firstErrorStatus     = 400
	lastErrorStatus      = 599
)

// RawErrorResponse is goast.raw-error-response: errors leave an API through
// one door, so a 4xx or 5xx answered directly by a handler outside the
// errors package is a finding unless the line carries a contract marker
// naming the envelope it mirrors. It needs the api capability.
func RawErrorResponse() model.Check {
	return check{spec: spec("raw-error-response", "api"), judge: judgeRawErrorResponse}
}

func judgeRawErrorResponse(t *target, _ *state) ([]model.Finding, error) {
	if path.Base(t.dir()) == errorsPackage {
		return nil, nil
	}
	file, fset, err := t.parsed()
	if err != nil {
		return nil, err
	}
	marked := commentLines(file, fset, contractMarker)
	var out []model.Finding
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !respondsWithError(call) || marked[fset.Position(call.Pos()).Line] {
			return true
		}
		message := "an error leaves the API outside the errors package: answer through its Problem Details door" +
			" or mark the line with a contract: comment naming the envelope it mirrors"
		out = append(out, t.finding(codeRawErrorResponse, call.Pos(), message))
		return true
	})
	return out, nil
}

// responseMethods are the handler methods that write a body with a status.
func responseMethods() []string {
	return []string{"JSON", "String", "Data", "AbortWithStatusJSON", "IndentedJSON"}
}

// respondsWithError reports a c.JSON-shaped call whose status is an error.
func respondsWithError(call *ast.CallExpr) bool {
	_, method, ok := selector(call.Fun)
	if !ok || !slices.Contains(responseMethods(), method) || len(call.Args) == 0 {
		return false
	}
	return errorStatus(call.Args[0])
}

// errorStatus reports an http.Status constant of the 4xx or 5xx classes or
// an integer literal in that range. The literal is read the way the
// compiler reads it, base prefix and digit separators included, so 0x1f4
// and 5_00 are 500 and 0500 is the octal 320.
func errorStatus(arg ast.Expr) bool {
	if pkg, name, ok := selector(arg); ok {
		return pkg == "http" && slices.Contains(errorStatusNames(), name)
	}
	lit, ok := arg.(*ast.BasicLit)
	if !ok || lit.Kind != token.INT {
		return false
	}
	code, err := strconv.ParseInt(lit.Value, 0, 64)
	return err == nil && code >= firstErrorStatus && code <= lastErrorStatus
}

// errorStatusNames are the net/http constants of the 4xx and 5xx classes.
func errorStatusNames() []string {
	return []string{
		"StatusBadRequest", "StatusUnauthorized", "StatusPaymentRequired", "StatusForbidden",
		"StatusNotFound", "StatusMethodNotAllowed", "StatusNotAcceptable", "StatusProxyAuthRequired",
		"StatusRequestTimeout", "StatusConflict", "StatusGone", "StatusLengthRequired",
		"StatusPreconditionFailed", "StatusRequestEntityTooLarge", "StatusRequestURITooLong",
		"StatusUnsupportedMediaType", "StatusRequestedRangeNotSatisfiable", "StatusExpectationFailed",
		"StatusTeapot", "StatusMisdirectedRequest", "StatusUnprocessableEntity", "StatusLocked",
		"StatusFailedDependency", "StatusTooEarly", "StatusUpgradeRequired", "StatusPreconditionRequired",
		"StatusTooManyRequests", "StatusRequestHeaderFieldsTooLarge", "StatusUnavailableForLegalReasons",
		"StatusInternalServerError", "StatusNotImplemented", "StatusBadGateway", "StatusServiceUnavailable",
		"StatusGatewayTimeout", "StatusHTTPVersionNotSupported", "StatusVariantAlsoNegotiates",
		"StatusInsufficientStorage", "StatusLoopDetected", "StatusNotExtended",
		"StatusNetworkAuthenticationRequired",
	}
}

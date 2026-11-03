package genmap

import (
	"cmp"
	"go/ast"
	"go/token"
	"slices"
	"strconv"
	"strings"
)

// route is one registered pattern and the function that serves it.
type route struct {
	method, path, target string
}

// line writes the route as "GET /plans -> plans".
func (rt route) line() string {
	return rt.method + " " + rt.path + " -> " + rt.target
}

// registrar reports whether a call name registers a route, and the method
// a router verb such as Get fixes.
func registrar(name string) (string, bool) {
	switch name {
	case "Handle", "HandleFunc", "Route", "Any", "Add":
		return "", true
	case "Get", "Post", "Put", "Patch", "Delete", "Head", "Options", "GET", "POST", "PUT", "PATCH", "DELETE":
		return strings.ToUpper(name), true
	}
	return "", false
}

// routesOf reads the route table of a package from its source: every call
// that registers a pattern with a handler and every literal that pairs a
// pattern with a handler, the patterns built from string constants.
func routesOf(p pkgSrc) []route {
	c := routeCollector{consts: stringConsts(p)}
	for _, s := range p.srcs {
		ast.Inspect(s.file, c.visit)
	}
	return c.out
}

// routeCollector gathers the routes of the nodes it visits.
type routeCollector struct {
	consts map[string]string
	out    []route
}

// visit reads a registration call or a pattern and handler literal.
func (c *routeCollector) visit(n ast.Node) bool {
	switch x := n.(type) {
	case *ast.CallExpr:
		sel, ok := x.Fun.(*ast.SelectorExpr)
		if !ok || len(x.Args) < 2 {
			return true
		}
		if verb, registers := registrar(sel.Sel.Name); registers {
			c.add(x.Args[0], verb, x.Args[len(x.Args)-1])
		}
	case *ast.CompositeLit:
		if len(x.Elts) >= 2 {
			c.add(elemValue(x.Elts, 0), "", elemValue(x.Elts, 1))
		}
	}
	return true
}

// add keeps a route once when the pattern evaluates and the handler names
// a function.
func (c *routeCollector) add(patternExpr ast.Expr, verb string, handler ast.Expr) {
	pattern, ok := stringValue(patternExpr, c.consts)
	if !ok {
		return
	}
	if rt, isRoute := splitRoute(pattern, verb, handlerName(handler)); isRoute && !slices.Contains(c.out, rt) {
		c.out = append(c.out, rt)
	}
}

// elemValue is the value of the i-th element of a composite literal.
func elemValue(elts []ast.Expr, i int) ast.Expr {
	if i >= len(elts) {
		return nil
	}
	if kv, ok := elts[i].(*ast.KeyValueExpr); ok {
		return kv.Value
	}
	return elts[i]
}

// splitRoute reads "GET /plans" or "/plans" as a route to a named target.
func splitRoute(pattern, verb, target string) (route, bool) {
	if target == "" {
		return route{}, false
	}
	method, rest, spaced := strings.Cut(pattern, " ")
	if spaced && method == strings.ToUpper(method) && strings.HasPrefix(strings.TrimSpace(rest), "/") {
		return route{method: method, path: strings.TrimSpace(rest), target: target}, true
	}
	if !strings.HasPrefix(pattern, "/") || strings.ContainsAny(pattern, " \n") {
		return route{}, false
	}
	return route{method: cmp.Or(verb, "*"), path: pattern, target: target}, true
}

// handlerName is the function a handler expression names: h.plans,
// plans, or the function a conversion or a middleware call wraps.
func handlerName(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.SelectorExpr:
		return x.Sel.Name
	case *ast.Ident:
		return x.Name
	case *ast.CallExpr:
		if len(x.Args) > 0 {
			return handlerName(x.Args[len(x.Args)-1])
		}
	}
	return ""
}

// stringConsts answers the string constants of a package, a constant
// built from others resolved, by name; three passes resolve a constant
// declared before the ones it is built from.
func stringConsts(p pkgSrc) map[string]string {
	out := map[string]string{}
	for range 3 {
		for _, s := range p.srcs {
			for _, vs := range valueSpecs(s.file, token.CONST) {
				resolveConsts(vs, out)
			}
		}
	}
	return out
}

// resolveConsts adds the string values of one spec that evaluate.
func resolveConsts(vs *ast.ValueSpec, out map[string]string) {
	for i, n := range vs.Names {
		if i >= len(vs.Values) {
			return
		}
		if v, ok := stringValue(vs.Values[i], out); ok {
			out[n.Name] = v
		}
	}
}

// stringValue evaluates a string literal, a constant or a concatenation.
func stringValue(e ast.Expr, consts map[string]string) (string, bool) {
	switch x := e.(type) {
	case *ast.BasicLit:
		if x.Kind != token.STRING {
			return "", false
		}
		v, err := strconv.Unquote(x.Value)
		return v, err == nil
	case *ast.Ident:
		v, ok := consts[x.Name]
		return v, ok
	case *ast.BinaryExpr:
		left, okLeft := stringValue(x.X, consts)
		right, okRight := stringValue(x.Y, consts)
		return left + right, okLeft && okRight && x.Op == token.ADD
	case *ast.ParenExpr:
		return stringValue(x.X, consts)
	}
	return "", false
}

// usecaseCalls answers the use cases a function calls Execute on, named by
// the field that holds each through the fields of the area's structs.
func usecaseCalls(fn *ast.FuncDecl, fields map[string]string) []string {
	var out []string
	ast.Inspect(fn, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != wordExecute {
			return true
		}
		holder := handlerName(sel.X)
		if uc := cmp.Or(fields[holder], holder); uc != "" && !slices.Contains(out, uc) {
			out = append(out, uc)
		}
		return true
	})
	return out
}

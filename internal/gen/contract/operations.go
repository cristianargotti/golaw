package contract

import (
	"fmt"
	"go/ast"
	"maps"
	"slices"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
)

const (
	// strictServer is the interface oapi-codegen writes with its strict
	// server option, one method per operation of the document.
	strictServer = "StrictServerInterface"
	// defaultTag is the tag oapi-codegen gives an operation without one.
	defaultTag = "default"
)

// httpMethods are the operation keys of an OpenAPI path item, in the order a
// search by operation id walks them; every other key of an item is skipped.
func httpMethods() []string {
	return []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}
}

// operationDoc is what a handler needs of one operation of the document.
type operationDoc struct {
	OperationID string   `yaml:"operationId"`
	Tags        []string `yaml:"tags"`
}

// pathItem keeps each key of a path item undecoded, so a parameters list or
// an extension beside the operations never fails the decode of the item.
type pathItem map[string]yaml.Node

// operations are the operations of a document by route and lower case method.
type operations map[string]map[string]operationDoc

// Operations answers one operation per method of the strict server in source
// order, its method and route from the route comment oapi-codegen writes, or
// by its operation id when the comment is absent, and its tag from the
// document. An operation the document lacks is refused, never guessed.
func (r Reader) Operations(server, openapi string) ([]genkind.Operation, error) {
	c, iface, err := r.strictServer(server)
	if err != nil {
		return nil, err
	}
	data, err := r.Snapshot.Read(openapi)
	if err != nil {
		return nil, fmt.Errorf("read openapi document %s: %w", openapi, err)
	}
	ops, err := document(openapi, data)
	if err != nil {
		return nil, err
	}
	out := make([]genkind.Operation, 0, len(iface.Methods.List))
	for _, field := range iface.Methods.List {
		if len(field.Names) == 0 {
			continue
		}
		op, err := c.operation(ops, field, openapi)
		if err != nil {
			return nil, err
		}
		out = append(out, op)
	}
	return out, nil
}

// strictServer reads and parses the oapi-codegen server file and finds the
// strict server interface in it.
func (r Reader) strictServer(server string) (contractFile, *ast.InterfaceType, error) {
	content, err := r.Snapshot.Read(server)
	if err != nil {
		return contractFile{}, nil, fmt.Errorf("read strict server %s: %w", server, err)
	}
	file, fset, err := parse(server, content)
	if err != nil {
		return contractFile{}, nil, err
	}
	for _, spec := range typeSpecs(file) {
		iface, ok := spec.spec.Type.(*ast.InterfaceType)
		if ok && spec.spec.Name.Name == strictServer {
			return contractFile{rel: server, fset: fset, content: content, file: file}, iface, nil
		}
	}
	return contractFile{}, nil, fmt.Errorf("%w: %s", ErrNoServer, server)
}

// operation matches one strict server method with its operation in the
// document.
func (c contractFile) operation(ops operations, field *ast.Field, openapi string) (genkind.Operation, error) {
	name, line := field.Names[0].Name, c.line(field.Names[0])
	method, route := routeOf(field.Doc)
	op, found := ops[route][strings.ToLower(method)]
	at := located{method: method, route: route, op: op}
	if route == "" {
		at, found = ops.byID(name)
	}
	if !found {
		return genkind.Operation{}, fmt.Errorf("%w: %s:%d operation %s is not in %s: run golaw gen after editing %s",
			genkind.ErrCannotJudge, c.rel, line, name, openapi, openapi)
	}
	tag := defaultTag
	if len(at.op.Tags) > 0 {
		tag = at.op.Tags[0]
	}
	return genkind.Operation{Name: name, Tag: tag, Method: strings.ToUpper(at.method), Route: at.route, Line: line}, nil
}

// located is one operation of the document with the method and route that
// hold it.
type located struct {
	method, route string
	op            operationDoc
}

// byID finds the operation whose operation id oapi-codegen spells as name,
// walking the routes in order so two runs answer the same match.
func (ops operations) byID(name string) (located, bool) {
	for _, route := range slices.Sorted(maps.Keys(ops)) {
		for _, method := range httpMethods() {
			op, ok := ops[route][method]
			if ok && goName(op.OperationID) == name {
				return located{method: method, route: route, op: op}, true
			}
		}
	}
	return located{}, false
}

// routeOf answers the method and route of the "(GET /plans/{id})" line
// oapi-codegen writes in the doc of each strict server method.
func routeOf(group *ast.CommentGroup) (string, string) {
	for line := range strings.SplitSeq(doc(group), "\n") {
		inner, ok := strings.CutPrefix(strings.TrimSpace(line), "(")
		if !ok {
			continue
		}
		inner, ok = strings.CutSuffix(inner, ")")
		method, route, split := strings.Cut(inner, " ")
		if ok && split && method != "" && route != "" {
			return method, route
		}
	}
	return "", ""
}

// goName spells an operation id the way oapi-codegen names its method: the
// first letter upper cased, and each separator dropped with the letter after
// it upper cased, so list_plans is ListPlans and getPlan is GetPlan.
func goName(operationID string) string {
	out := make([]rune, 0, len(operationID))
	upper := true
	for _, r := range operationID {
		if strings.ContainsRune("-_. ", r) {
			upper = true
			continue
		}
		if upper {
			r = unicode.ToUpper(r)
			upper = false
		}
		out = append(out, r)
	}
	return string(out)
}

// document decodes the operations of an OpenAPI document by route and
// method, every other key of a path item left undecoded.
func document(openapi string, data []byte) (operations, error) {
	var raw struct {
		Paths map[string]pathItem `yaml:"paths"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("read %s: %w", openapi, err)
	}
	ops := make(operations, len(raw.Paths))
	for route, item := range raw.Paths {
		ops[route] = make(map[string]operationDoc)
		for _, method := range httpMethods() {
			node, ok := item[method]
			if !ok {
				continue
			}
			var op operationDoc
			if err := node.Decode(&op); err != nil {
				return nil, fmt.Errorf("read %s: %s %s: %w", openapi, method, route, err)
			}
			ops[route][method] = op
		}
	}
	return ops, nil
}

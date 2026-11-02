package handler

import (
	"fmt"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/render"
	"github.com/cristianargotti/golaw/internal/genheader"
)

const (
	// routesPerFile keeps a route file, one pattern and one cause line per
	// operation, under the 300 line limit with its header and imports, so a
	// document splits first.
	routesPerFile = 120
	stdContext    = "context"
	stdErrors     = "errors"
	stdHTTP       = "net/http"
	stdTesting    = "testing"
)

// page is what the handler templates read; only Chunk, Patterns and Calls
// vary by file.
type page struct {
	Package, API, AreaList, Params, Fields      string
	Areas                                       []area
	Chunk                                       int
	Patterns, Calls                             []string
	Ops                                         []operation
	StatusHole, StatusConstraint, StatusMessage string
}

// spec is one file to render: its template, its path and its imports.
type spec struct {
	template, path string
	page           page
	imports        []genkind.Import
}

func pageOf(v view) page {
	names, params, fields := make([]string, len(v.Areas)), make([]string, len(v.Areas)), make([]string, len(v.Areas))
	for i, a := range v.Areas {
		names[i] = a.Name
		params[i] = a.Field + " " + a.Name + ".UseCases"
		fields[i] = a.Field + ": " + a.Field
	}
	hole, constraint, message := statusHole(v)
	return page{
		Package: v.Package, API: v.API, AreaList: listOf(names),
		Params: strings.Join(params, ", "), Fields: strings.Join(fields, ", "),
		Areas: v.Areas, Ops: v.Ops,
		StatusHole: hole, StatusConstraint: constraint, StatusMessage: message,
	}
}

// listOf joins names the way a sentence reads them: a, b and c.
func listOf(names []string) string {
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// regenerable renders the files golaw owns of one handler instance, the
// ones the header marks and every regeneration overwrites.
func regenerable(v view, kitVersion string) ([]genkind.File, error) {
	header := genheader.Header{Version: kitVersion, Kind: genkind.KindHandler, Template: version}
	return renderAll(v.Module, regenerableSpecs(v), header)
}

func regenerableSpecs(v view) []spec {
	p := pageOf(v)
	areaImports := make([]genkind.Import, 0, len(v.Areas))
	var fakeImports []genkind.Import
	for _, a := range v.Areas {
		areaImports = append(areaImports, a.Import)
		for _, f := range a.Fakes {
			fakeImports = append(fakeImports, f.Import)
		}
	}
	tests := slices.Concat(std(stdContext, stdErrors, stdHTTP, "net/http/httptest", "reflect", "strings", stdTesting),
		[]genkind.Import{v.Self, v.Support}, areaImports, fakeImports)
	return append([]spec{
		{"handler.go.tmpl", path.Join(v.Dir, "zz_generated_handler.go"), p, slices.Concat(std(stdErrors, stdHTTP), []genkind.Import{v.APIGen}, areaImports)},
		{"handler_internal_test.go.tmpl", path.Join(v.Dir, "zz_generated_handler_internal_test.go"), p, std(stdErrors, "fmt", stdHTTP, stdTesting)},
		{"handler_test.go.tmpl", path.Join(v.Dir, "zz_generated_handler_test.go"), p, tests},
	}, routeSpecs(v, p)...)
}

// routeSpecs splits the route table into files of routesPerFile patterns,
// the first one unnumbered and the next ones numbered from 2.
func routeSpecs(v view, p page) []spec {
	routes := chunk(v.Ops, routesPerFile)
	specs := make([]spec, 0, len(routes))
	for i, part := range routes {
		n, name := i+1, "zz_generated_handler_routes_test.go"
		if n > 1 {
			name = "zz_generated_handler_routes_" + strconv.Itoa(n) + "_test.go"
		}
		file := p
		file.Chunk, file.Patterns, file.Calls = n, make([]string, len(part)), make([]string, len(part))
		for j, op := range part {
			file.Patterns[j], file.Calls[j] = op.Pattern, op.Name
		}
		specs = append(specs, spec{"routes_test.go.tmpl", path.Join(v.Dir, name), file, std(stdContext, stdTesting)})
	}
	return specs
}

// renderAll renders every spec under one header, the zero header for owned
// files, and answers the files in path order so a run never reorders them.
func renderAll(module string, specs []spec, header genheader.Header) ([]genkind.File, error) {
	templates, err := render.Templates(genkind.KindHandler, version)
	if err != nil {
		return nil, fmt.Errorf("handler templates: %w", err)
	}
	// A value, because goast.goleak reads a call of a selector Go as a goroutine start.
	renderGo := render.Go
	files := make([]genkind.File, 0, len(specs))
	for i := range specs {
		s := &specs[i]
		content, err := renderGo(templates, s.template, s.page, header, s.imports, module)
		if err != nil {
			return nil, fmt.Errorf("handler template %s: %w", s.template, err)
		}
		files = append(files, genkind.File{Path: s.path, Content: content})
	}
	slices.SortFunc(files, func(a, b genkind.File) int { return strings.Compare(a.Path, b.Path) })
	return files, nil
}

// chunk never answers an empty run, so no route file renders without a pattern.
func chunk[T any](items []T, size int) [][]T {
	var out [][]T
	for part := range slices.Chunk(items, size) {
		out = append(out, part)
	}
	return out
}

// std spells standard library imports, which never carry a name.
func std(paths ...string) []genkind.Import {
	out := make([]genkind.Import, len(paths))
	for i, p := range paths {
		out[i] = genkind.Import{Path: p}
	}
	return out
}

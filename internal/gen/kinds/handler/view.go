package handler

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/holes"
	"github.com/cristianargotti/golaw/internal/model"
)

const (
	version    = 1
	serverFile = "server.gen.go"
	// NewHandler takes one UseCases value per area, and go:S107 stops a
	// function at seven parameters, so an eighth area would fail the gate.
	maxAreas    = 7
	handlerType = "Handler"
)

type view struct {
	Module, Package, Dir, API, Server string
	APIGen, Support, Self             genkind.Import
	Areas                             []area
	Ops                               []operation
}

type area struct {
	Name string
	// Field is the parameter and field name of the area: uc alone, so one
	// area reads naturally, and uc<Area> with several, so each stays apart.
	Field  string
	Import genkind.Import
	Fakes  []fake
}

type fake struct {
	Port, Ident string
	Import      genkind.Import
}

type operation struct {
	Name, Pattern, Stem, Hole, Constraint, Message string
}

// newView judges a golaw new handler request against the generate block,
// the manifest and the snapshot before any template renders, so a refusal
// names the command that fixes it instead of leaving half a handler.
func newView(in genkind.Input, req genkind.Request) (view, error) {
	if err := checkName(req.Name); err != nil {
		return view{}, err
	}
	pkg, api := req.Name.Area, req.API
	if api == "" {
		api = in.Policy.ThirdParty.OpenAPI
	}
	if api != in.Policy.ThirdParty.OpenAPI {
		return view{}, fmt.Errorf("%w: --api %s is not the document golaw.yml names in generate.third_party.openapi (%s)",
			genkind.ErrRequest, api, in.Policy.ThirdParty.OpenAPI)
	}
	server := path.Join(in.Policy.ThirdParty.OpenAPIOut, serverFile)
	if err := checkFree(in, pkg, server); err != nil {
		return view{}, err
	}
	if in.Snapshot.Class(server) == model.ClassUnknown {
		return view{}, fmt.Errorf("%w: %s is not written yet: run golaw gen to write it from %s", genkind.ErrRequest, server, api)
	}
	return build(in, pkg, api, server, req.Usecases)
}

// build derives the view of a handler from its areas and the operations of
// the server file; Regenerate calls it over a manifest entry as well, which
// is why it takes the judged names instead of a request.
func build(in genkind.Input, pkg, api, server string, names []string) (view, error) {
	areas, err := areasOf(in, pkg, names)
	if err != nil {
		return view{}, err
	}
	ops, err := in.Contracts.Operations(server, api)
	if err != nil {
		return view{}, fmt.Errorf("handler %s: %w", pkg, err)
	}
	if len(ops) == 0 {
		return view{}, fmt.Errorf("%w: %s declares no operation: add one to %s and run golaw gen", genkind.ErrRequest, server, api)
	}
	fields := make([]string, len(areas))
	for i, a := range areas {
		fields[i] = a.Field
	}
	dir := dirOf(in, pkg)
	return view{
		Module: in.Module, Package: pkg, Dir: dir, API: api, Server: server,
		// The name is explicit because the last path element, gen, differs
		// from the package clause oapi-codegen writes.
		APIGen:  genkind.Import{Name: "apigen", Path: in.Module + "/" + in.Policy.ThirdParty.OpenAPIOut},
		Support: genkind.Import{Path: in.Module + "/" + in.Policy.Support},
		Self:    genkind.Import{Path: in.Module + "/" + dir},
		Areas:   areas,
		Ops:     operationsOf(pkg, fields, ops),
	}, nil
}

func dirOf(in genkind.Input, pkg string) string {
	return strings.ReplaceAll(in.Policy.Layout.Handler, "{package}", pkg)
}

// operationsOf keeps the order the contract reader answers, the source
// order of the server interface, so the owned stubs follow the document.
func operationsOf(pkg string, fields []string, ops []genkind.Operation) []operation {
	out := make([]operation, 0, len(ops))
	for _, op := range ops {
		hole := holes.ID(pkg, op.Name)
		action := pkg + ": " + strings.ReplaceAll(strings.TrimPrefix(hole, pkg+"."), "-", " ")
		constraint := constraintOf(op.Name, fields, action)
		out = append(out, operation{
			Name: op.Name, Pattern: op.Method + " " + op.Route, Stem: stem(op.Tag),
			Hole: hole, Constraint: constraint, Message: "hole " + hole + " is open: " + constraint,
		})
	}
	return out
}

// constraintOf is the one line a hole carries; it holds no format verb and
// no newline, so the red test passes it to t.Fatal without a vet finding.
func constraintOf(name string, fields []string, action string) string {
	calls := make([]string, len(fields))
	for i, field := range fields {
		calls[i] = "h." + field
	}
	return "map the request to the input of a use case of " + strings.Join(calls, " or ") +
		" and call its Execute; answer apigen." + name +
		" responses, the declared status of statusOf(err, statusRules()...) for an error," +
		" and wrap every other error with gensupport.Prefix(" + strconv.Quote(action) + ")"
}

func statusHole(v view) (string, string, string) {
	names := make([]string, len(v.Areas))
	for i, a := range v.Areas {
		names[i] = a.Name
	}
	id := holes.ID(v.Package, "StatusRules")
	constraint := "list each domain error the use cases of " + strings.Join(names, " and ") +
		" return with the HTTP status it answers, as {err: <sentinel>, status: <http.Status constant>}"
	return id, constraint, "hole " + id + " is open: " + constraint
}

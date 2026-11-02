package wiring

import (
	"cmp"
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
)

// The mux package every root imports, and how a call reads the infra value:
// an area whose calls never read it takes no parameter, so none is unused.
const (
	httpImport = "net/http"
	infraRead  = "infra."
)

type (
	dep     struct{ Port, Call string }
	handler struct{ Qualifier, Ctor, Args string }
	field   struct{ Name, Type string }
)

type area struct {
	Name, Func, Var, Qualifier, Ctor string
	Deps                             []dep
	Used, Param                      bool
}

// composition is what the regenerable root template reads.
type composition struct {
	Areas     []area
	Handlers  []handler
	Infra     []field
	UsesInfra bool
	Imports   []genkind.Import
}

// builder collects one root; a binding it cannot place is a decision on file.
type builder struct {
	file      string
	areas     map[string]*area
	fields    map[string]field
	imports   map[string]genkind.Import
	decisions []genkind.Decision
	handlers  []handler
}

// compose turns the bindings of every entry into the root of one main package,
// sorted first so the bytes never depend on the order entries were read in.
func compose(bindings []genkind.Binding, file string) (composition, []genkind.Decision, error) {
	sorted := slices.Clone(bindings)
	slices.SortFunc(sorted, func(a, b genkind.Binding) int {
		return cmp.Or(cmp.Compare(a.Kind, b.Kind), cmp.Compare(a.Area, b.Area), cmp.Compare(a.Port, b.Port), cmp.Compare(a.Call, b.Call))
	})
	b := &builder{
		file: file, areas: map[string]*area{}, fields: map[string]field{},
		imports: map[string]genkind.Import{httpImport: {Path: httpImport}},
	}
	for _, binding := range sorted {
		if binding.Kind == genkind.KindUsecase {
			b.usecase(binding)
		}
	}
	for _, binding := range sorted {
		if binding.Kind != genkind.KindUsecase && binding.Kind != genkind.KindHandler {
			if err := b.adapter(binding); err != nil {
				return composition{}, nil, err
			}
		}
	}
	b.mount(sorted)
	return b.composition(), b.decisions, nil
}

// usecase opens the area of a use case binding; a second binding of the
// same area adds nothing, because one area has one constructor.
func (b *builder) usecase(binding genkind.Binding) {
	if _, ok := b.areas[binding.Area]; ok {
		return
	}
	b.areas[binding.Area] = &area{
		Name: binding.Area, Func: "wire" + upperFirst(binding.Area), Var: binding.Area + "UC",
		Qualifier: qualifier(binding.Import), Ctor: binding.Call,
	}
	b.use(binding.Import)
}

// adapter binds one port of an area to its call; an area no use case opened
// leaves it a decision, as a Deps field without its use case has no struct.
func (b *builder) adapter(binding genkind.Binding) error {
	a, ok := b.areas[binding.Area]
	if !ok {
		b.decide(fmt.Sprintf("%s binds port %s of area %s, which no use case consumes: create the use case with golaw new usecase and --ports %s",
			binding.Call, binding.Port, binding.Area, binding.Port))
		return nil
	}
	if slices.ContainsFunc(a.Deps, func(d dep) bool { return d.Port == binding.Port }) {
		return fmt.Errorf("%w: port %s of area %s is bound twice", genkind.ErrRequest, binding.Port, binding.Area)
	}
	for _, need := range binding.Needs {
		if have, seen := b.fields[need.Field]; seen && have.Type != need.Type {
			return fmt.Errorf("%w: infra field %s is %s and %s", genkind.ErrRequest, need.Field, have.Type, need.Type)
		}
		b.fields[need.Field] = field{Name: need.Field, Type: need.Type}
		b.use(need.Import)
	}
	a.Deps = append(a.Deps, dep{Port: binding.Port, Call: binding.Call})
	a.Param = a.Param || strings.Contains(binding.Call, infraRead)
	b.use(binding.Import)
	return nil
}

// mount groups the handler bindings by constructor, one per area it takes, in
// the order of their first binding; a handler taking an area without a use
// case is left out whole, since its constructor needs every argument.
func (b *builder) mount(sorted []genkind.Binding) {
	var calls []string
	byCall := map[string][]genkind.Binding{}
	for _, binding := range sorted {
		if binding.Kind == genkind.KindHandler {
			if _, seen := byCall[binding.Call]; !seen {
				calls = append(calls, binding.Call)
			}
			byCall[binding.Call] = append(byCall[binding.Call], binding)
		}
	}
	for _, call := range calls {
		// Sorted by area, a repeated area is adjacent: one argument per area.
		b.handler(slices.CompactFunc(byCall[call], func(x, y genkind.Binding) bool { return x.Area == y.Area }))
	}
}

func (b *builder) handler(group []genkind.Binding) {
	vars := make([]string, 0, len(group))
	for _, binding := range group {
		if a, ok := b.areas[binding.Area]; ok {
			vars = append(vars, a.Var)
			continue
		}
		b.decide(fmt.Sprintf("%s takes area %s, which has no use case: create the use case with golaw new usecase", binding.Call, binding.Area))
	}
	if len(vars) < len(group) {
		return
	}
	for _, binding := range group {
		b.areas[binding.Area].Used = true
	}
	b.handlers = append(b.handlers, handler{Qualifier: qualifier(group[0].Import), Ctor: group[0].Call, Args: strings.Join(vars, ", ")})
	b.use(group[0].Import)
}

func (b *builder) decide(text string) {
	b.decisions = append(b.decisions, genkind.Decision{Path: b.file, Text: text})
}

// use records one import by path; an empty path names one already imported.
func (b *builder) use(imp genkind.Import) {
	if imp.Path != "" {
		b.imports[imp.Path] = imp
	}
}

// composition sorts areas by name, deps by port, fields by name, imports by path.
func (b *builder) composition() composition {
	out := composition{Handlers: b.handlers}
	for _, name := range slices.Sorted(maps.Keys(b.areas)) {
		a := *b.areas[name]
		slices.SortFunc(a.Deps, func(x, y dep) int { return cmp.Compare(x.Port, y.Port) })
		out.Areas = append(out.Areas, a)
		out.UsesInfra = out.UsesInfra || a.Param
	}
	for _, name := range slices.Sorted(maps.Keys(b.fields)) {
		out.Infra = append(out.Infra, b.fields[name])
	}
	for _, p := range slices.Sorted(maps.Keys(b.imports)) {
		out.Imports = append(out.Imports, b.imports[p])
	}
	return out
}

// qualifier is the name a call reaches the package by: the import's name,
// or the last element of its path the way Go names an unnamed import.
func qualifier(imp genkind.Import) string {
	if imp.Name != "" {
		return imp.Name
	}
	return path.Base(imp.Path)
}

func upperFirst(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}

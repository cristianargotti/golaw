package handler

import (
	"fmt"
	"go/token"
	"path"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/model"
)

// fixedImports are the identifiers the generated handler files import or
// declare whatever the request, so neither the package nor an area nor a
// fake may take one of them.
func fixedImports() []string {
	return []string{"apigen", stdContext, "errors", "fmt", "gensupport", "http", "httptest", "strings", "testing"}
}

// reserved is a function, never a package variable, so every call answers a
// set the caller may extend while it names the fakes.
func reserved(pkg string, areas []string) map[string]bool {
	taken := map[string]bool{pkg: true}
	for _, name := range slices.Concat(fixedImports(), areas) {
		taken[name] = true
	}
	return taken
}

func goodName(name string) bool {
	return name != "" && name[0] >= 'a' && name[0] <= 'z' && !token.IsKeyword(name) &&
		strings.Trim(name, "abcdefghijklmnopqrstuvwxyz0123456789") == ""
}

func checkName(n genkind.Name) error {
	if !goodName(n.Area) || slices.Contains(fixedImports(), n.Area) {
		return fmt.Errorf("%w: handler name %q is not a free package name: use lower case letters and digits, for example plansapi", genkind.ErrRequest, n.Area)
	}
	if n.Type != "" && n.Type != handlerType {
		return fmt.Errorf("%w: a handler is named by its package, %s, not %s.%s", genkind.ErrRequest, n.Area, n.Area, n.Type)
	}
	return nil
}

// checkFree refuses a second handler over a name or a server interface,
// because a second instance would overwrite owned code or serve the same
// routes twice.
func checkFree(in genkind.Input, pkg, server string) error {
	if e, ok := in.Manifest.Lookup(genkind.KindHandler, pkg+"."+handlerType); ok {
		return fmt.Errorf("%w: handler %s exists: %s", genkind.ErrExists, pkg, strings.Join(e.Regenerable, ", "))
	}
	for i := range in.Manifest.Entries {
		if e := &in.Manifest.Entries[i]; e.Kind == genkind.KindHandler && slices.ContainsFunc(e.Contracts, func(c manifest.Contract) bool { return c.Path == server }) {
			return fmt.Errorf("%w: %s is served by handler %s: one handler per server interface", genkind.ErrExists, server, e.Area())
		}
	}
	if file := path.Join(dirOf(in, pkg), "zz_generated_handler.go"); in.Snapshot.Class(file) != model.ClassUnknown {
		return fmt.Errorf("%w: %s exists without a manifest entry: remove it or run golaw gen", genkind.ErrExists, file)
	}
	return nil
}

func areasOf(in genkind.Input, pkg string, names []string) ([]area, error) {
	if len(names) == 0 {
		return nil, fmt.Errorf("%w: --usecases names no area: name the areas whose use cases the handler calls, for example --usecases growth", genkind.ErrRequest)
	}
	if len(names) > maxAreas {
		return nil, fmt.Errorf("%w: %d areas pass the limit of %d parameters of NewHandler: split the API into handlers of fewer areas", genkind.ErrRequest, len(names), maxAreas)
	}
	if err := checkAreas(pkg, names); err != nil {
		return nil, err
	}
	sorted := slices.Sorted(slices.Values(names))
	fields := fieldNames(sorted)
	taken := reserved(pkg, sorted)
	areas := make([]area, 0, len(sorted))
	for i, name := range sorted {
		a, err := areaOf(in, name, fields[i], taken)
		if err != nil {
			return nil, err
		}
		areas = append(areas, a)
	}
	return areas, nil
}

func checkAreas(pkg string, names []string) error {
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		switch {
		case !goodName(name) || slices.Contains(fixedImports(), name):
			return fmt.Errorf("%w: area %q is not a free package name: use lower case letters and digits, for example growth", genkind.ErrRequest, name)
		case seen[name]:
			return fmt.Errorf("%w: area %s is named twice in --usecases: name it once", genkind.ErrRequest, name)
		case name == pkg:
			return fmt.Errorf("%w: area %s is named like the handler package: name the handler after its API, for example %sapi", genkind.ErrRequest, name, name)
		}
		seen[name] = true
	}
	return nil
}

// areaOf names the fakes of an area over one taken set shared by every
// area, so a fake package named like an import of the generated test or
// like another area's fake is aliased, never shadowed.
func areaOf(in genkind.Input, name, field string, taken map[string]bool) (area, error) {
	ports, err := portsOf(in, name)
	if err != nil {
		return area{}, err
	}
	fakes := make([]fake, 0, len(ports))
	for _, port := range ports {
		if _, ok := in.Manifest.Lookup(genkind.KindPort, name+"."+port); !ok {
			return area{}, fmt.Errorf("%w: port %s.%s has no generated fake: run golaw new port %s.%s first", genkind.ErrRequest, name, port, name, port)
		}
		pkg := strings.ToLower(port) + "test"
		f := fake{Port: port, Ident: pkg, Import: genkind.Import{
			Path: in.Module + "/" + strings.ReplaceAll(in.Policy.Layout.Port, "{area}", name) + "/" + pkg,
		}}
		if taken[pkg] {
			f.Ident = name + pkg
			f.Import.Name = f.Ident
		}
		taken[f.Ident] = true
		fakes = append(fakes, f)
	}
	return area{
		Name: name, Field: field, Fakes: fakes,
		Import: genkind.Import{Path: in.Module + "/" + strings.ReplaceAll(in.Policy.Layout.Usecase, "{area}", name)},
	}, nil
}

// portsOf answers the union of the ports the use cases of an area consume,
// sorted, so the fakes of the generated test do not move between runs.
func portsOf(in genkind.Input, name string) ([]string, error) {
	var ports []string
	found := false
	for i := range in.Manifest.Entries {
		e := &in.Manifest.Entries[i]
		if e.Kind != genkind.KindUsecase || e.Area() != name {
			continue
		}
		found = true
		for _, port := range e.Ports {
			if !slices.Contains(ports, port) {
				ports = append(ports, port)
			}
		}
	}
	if !found {
		return nil, fmt.Errorf("%w: area %s has no use case: run golaw new usecase %s.<Verb><Object> first", genkind.ErrRequest, name, name)
	}
	slices.Sort(ports)
	return ports, nil
}

func fieldNames(areas []string) []string {
	if len(areas) == 1 {
		return []string{"uc"}
	}
	fields := make([]string, len(areas))
	for i, name := range areas {
		first, size := utf8.DecodeRuneInString(name)
		fields[i] = "uc" + strings.ToUpper(string(first)) + name[size:]
	}
	return fields
}

// stem names the owned file of a tag so it never meets the other owned
// names of the handler: status.go, <stem>_test.go and the chunks
// <stem>_<n>.go.
func stem(tag string) string {
	s := strings.Trim(strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, strings.ToLower(tag)), "_")
	switch {
	case s == "":
		return "default"
	case s[0] >= '0' && s[0] <= '9' || strings.HasPrefix(s, "zz_"):
		s = "api_" + s
	}
	if s == "status" || s == "handler" || strings.HasSuffix(s, "_test") || numbered(s) {
		s += "_api"
	}
	return s
}

// numbered reports a stem ending in _ followed by digits only, the shape of
// a chunk name.
func numbered(s string) bool {
	i := strings.LastIndexByte(s, '_')
	return i >= 0 && i < len(s)-1 && strings.Trim(s[i+1:], "0123456789") == ""
}

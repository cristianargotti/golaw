package usecase

import (
	"cmp"
	"fmt"
	"go/token"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/genheader"
	"github.com/cristianargotti/golaw/internal/model"
)

// maxUsecases is the number of NewUseCases lines that keep it within the
// 40 line law with its return and its closing brace.
const maxUsecases = 36

// areaToken is the placeholder of the port and use case layouts.
const areaToken = "{area}"

// directory is the consuming package of an area. The use case's Deps names
// the ports unqualified, so the ports must be declared in the same package.
func directory(layout model.GenerateLayout, area string) (string, error) {
	dir := strings.ReplaceAll(layout.Usecase, areaToken, area)
	if strings.ReplaceAll(layout.Port, areaToken, area) != dir {
		return "", fmt.Errorf("%w: generate.layout.port and generate.layout.usecase must name the same directory for %s: ports are declared by the consuming package", genkind.ErrRequest, area)
	}
	return dir, nil
}

// validate refuses a request no use case can be rendered from.
func validate(req genkind.Request) error {
	if !packageName(req.Name.Area) || !exported(req.Name.Type) {
		return fmt.Errorf("%w: %s is not <area>.<Type>: the area is a lower case package name and the type an exported Go name", genkind.ErrRequest, req.Name)
	}
	if len(req.Ports) == 0 {
		return fmt.Errorf("%w: golaw new usecase needs --ports: the use case consumes at least one port", genkind.ErrRequest)
	}
	for i, port := range req.Ports {
		if !exported(port) {
			return fmt.Errorf("%w: %s is not an exported Go name", genkind.ErrRequest, port)
		}
		if slices.Contains(req.Ports[:i], port) {
			return fmt.Errorf("%w: usecase %s names port %s twice", genkind.ErrRequest, req.Name, port)
		}
	}
	for _, word := range []string{req.In, req.Out} {
		if word != "" && !exported(word) {
			return fmt.Errorf("%w: %s is not an exported Go name", genkind.ErrRequest, word)
		}
	}
	return nil
}

// exported admits one exported Go identifier, never a qualified name.
func exported(word string) bool {
	return token.IsIdentifier(word) && token.IsExported(word)
}

// packageName admits the lower case letters and digits a package clause
// and an import path element both accept.
func packageName(area string) bool {
	return token.IsIdentifier(area) && strings.IndexFunc(area, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	}) < 0
}

// resolve answers the entry the request writes. The input and output of an
// existing entry are fixed by its owned Execute, which the kit never edits.
func resolve(m manifest.Manifest, req genkind.Request, dir string) (manifest.Entry, error) {
	e := newEntry(req, dir)
	old, ok := m.Lookup(genkind.KindUsecase, req.Name.String())
	if !ok {
		e.In = cmp.Or(req.In, req.Name.Type+"Input")
		e.Out = cmp.Or(req.Out, req.Name.Type+"Output")
		return e, nil
	}
	e.In, e.Out = cmp.Or(req.In, old.In), cmp.Or(req.Out, old.Out)
	if e.In != old.In || e.Out != old.Out {
		return manifest.Entry{}, fmt.Errorf("%w: usecase %s reads %s and answers %s, fixed by %s: keep --in and --out or change Execute by hand", genkind.ErrRequest, req.Name, old.In, old.Out, e.Owned[0])
	}
	if slices.Equal(old.Ports, e.Ports) {
		return manifest.Entry{}, fmt.Errorf("%w: usecase %s: %s", genkind.ErrExists, req.Name, e.Owned[0])
	}
	return e, nil
}

// newEntry is the entry of a use case in dir without its input and output.
func newEntry(req genkind.Request, dir string) manifest.Entry {
	stem := dir + "/" + snake(req.Name.Type)
	regenerable := []string{dir + "/zz_generated_" + snake(req.Name.Type) + ".go", dir + "/" + areaFile, dir + "/" + areaTestFile}
	slices.Sort(regenerable)
	return manifest.Entry{
		Kind:        genkind.KindUsecase,
		Template:    genheader.Header{Kind: genkind.KindUsecase, Template: templateVersion}.Ref(),
		Name:        req.Name.String(),
		Ports:       slices.Clone(req.Ports),
		Regenerable: regenerable,
		Owned:       []string{stem + ".go", stem + "_test.go"},
	}
}

// siblings answers the use case entries of the area once e is in place,
// sorted by name, and refuses an area NewUseCases could not hold.
func siblings(m manifest.Manifest, e manifest.Entry) ([]manifest.Entry, error) {
	var out []manifest.Entry
	entries := m.With(e).Entries
	for i := range entries {
		if entries[i].Kind == genkind.KindUsecase && entries[i].Area() == e.Area() {
			out = append(out, entries[i])
		}
	}
	if len(out) > maxUsecases {
		return nil, fmt.Errorf("%w: %s would hold %d use cases, over the 36 NewUseCases keeps within 40 lines: split the area", genkind.ErrRequest, e.Area(), len(out))
	}
	return out, nil
}

// needs asks for every port without an entry, except the port whose own
// request asked for this use case, so a chain of needs ends after one step.
func needs(m manifest.Manifest, req genkind.Request, e manifest.Entry) []genkind.Need {
	var out []genkind.Need
	for _, port := range e.Ports {
		name := genkind.Name{Area: req.Name.Area, Type: port}
		if _, ok := m.Lookup(genkind.KindPort, name.String()); ok || name == req.Port {
			continue
		}
		out = append(out, genkind.Need{Kind: genkind.KindPort, Request: genkind.Request{Name: name, Usecase: req.Name.Type, Port: req.Name, Ports: e.Ports}})
	}
	return out
}

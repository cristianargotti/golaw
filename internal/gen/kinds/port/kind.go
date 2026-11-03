package port

import (
	"context"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/genheader"
)

// kind is the port kind; it holds no state, so one value serves every run.
type kind struct{}

// Kind answers the port kind (M7.3.2).
func Kind() genkind.Kind { return kind{} }

// Name answers genkind.KindPort.
func (kind) Name() string { return genkind.KindPort }

// Template answers the version of the embedded port templates.
func (kind) Template() int { return templateVersion }

// New writes a port's test package and asks for its consuming use case. A
// port the area does not declare yet gets its interface stub, so the agent
// declares the methods and the hook regenerates the rest; a port the area
// already declares by hand is adopted: only its test package is written.
func (kind) New(_ context.Context, in genkind.Input, req genkind.Request) (genkind.Output, error) {
	if err := validate(req); err != nil {
		return genkind.Output{}, err
	}
	if err := consumer(in.Manifest, req); err != nil {
		return genkind.Output{}, err
	}
	dir := strings.ReplaceAll(in.Policy.Layout.Port, "{area}", req.Name.Area)
	if _, ok := in.Manifest.Lookup(genkind.KindPort, req.Name.String()); ok {
		fake := testDir(genkind.Port{Name: req.Name.Type, Dir: dir}) + "/" + fakeFile
		return genkind.Output{}, fmt.Errorf("%w: port %s: %s", genkind.ErrExists, req.Name, fake)
	}
	c, err := locate(in.Snapshot, dir, req.Name.Area, req.Name.Type)
	if err != nil {
		return genkind.Output{}, err
	}
	p, err := declaredPort(in, req, c, dir)
	if err != nil {
		return genkind.Output{}, err
	}
	out, v, err := generated(in, p, req.Name.String())
	if err != nil {
		return genkind.Output{}, err
	}
	cases, err := newCases(in, p, v)
	if err != nil {
		return genkind.Output{}, err
	}
	out.Owned = []genkind.File{cases}
	if !c.Declared {
		stub, err := stubPart(in, c, v)
		if err != nil {
			return genkind.Output{}, err
		}
		out.Owned, out.Appends = append(out.Owned, stub.Owned...), stub.Appends
	}
	out.Needs = needs(in.Manifest, req)
	return out, nil
}

// declaredPort reads the port where its area declares it, or shapes the
// stub of a port the area does not declare yet.
func declaredPort(in genkind.Input, req genkind.Request, c contractFile, dir string) (genkind.Port, error) {
	if !c.Declared {
		// A port the same use case run names is no earlier stub left
		// unfilled: the person asked for both at once, and each opens its
		// own hole.
		empty := slices.DeleteFunc(slices.Clone(c.empty), func(name string) bool { return slices.Contains(req.Ports, name) })
		if len(empty) > 0 {
			return genkind.Port{}, fmt.Errorf("%w: %s already declares the port %s without methods: declare its methods before golaw new port %s",
				genkind.ErrRequest, dir, empty[0], req.Name)
		}
		return genkind.Port{Name: req.Name.Type, Package: c.Package, Path: c.Path, Dir: dir}, nil
	}
	p, err := in.Contracts.Port(c.Path, req.Name.Type)
	if err != nil {
		return genkind.Port{}, fmt.Errorf("port %s: %w", req.Name, err)
	}
	return p, twin(in, req, p)
}

// Regenerate renders the regenerable files of an existing port again and
// the owned code its methods lack; a port no longer declared is a decision,
// because only the agent knows whether it moved or is gone.
func (kind) Regenerate(_ context.Context, in genkind.Input, entry manifest.Entry) (genkind.Output, error) {
	if len(entry.Contracts) == 0 {
		return genkind.Output{}, fmt.Errorf("%w: entry %s names no contract", genkind.ErrRequest, entry.Name)
	}
	contract := entry.Contracts[0]
	dir := path.Dir(contract.Path)
	c, err := locate(in.Snapshot, dir, entry.Area(), contract.Symbol)
	if err != nil {
		return genkind.Output{}, err
	}
	if !c.Declared {
		return gone(entry, contract, dir), nil
	}
	p, err := in.Contracts.Port(c.Path, contract.Symbol)
	if err != nil {
		return genkind.Output{}, fmt.Errorf("port %s: %w", entry.Name, err)
	}
	out, v, err := generated(in, p, entry.Name)
	if err != nil {
		return genkind.Output{}, err
	}
	part, err := regenerateCases(in, p, v)
	if err != nil {
		return genkind.Output{}, err
	}
	out.Owned, out.Appends, out.Decisions = part.Owned, part.Appends, part.Decisions
	return out, nil
}

// gone is the decision of a port its contract no longer declares.
func gone(entry manifest.Entry, contract manifest.Contract, dir string) genkind.Output {
	test := testDir(genkind.Port{Name: contract.Symbol, Dir: dir})
	return genkind.Output{Entry: entry, Decisions: []genkind.Decision{{Path: contract.Path, Line: 1, Text: "port " + entry.Name +
		" is no longer declared in " + contract.Path + ": declare it again, or delete " + test + " and the port's use in the use cases of " + entry.Area()}}}
}

// generated renders the regenerable files and the entry that lists them.
func generated(in genkind.Input, p genkind.Port, name string) (genkind.Output, view, error) {
	v, err := newView(p)
	if err != nil {
		return genkind.Output{}, view{}, err
	}
	files, err := regenerable(in, p, v)
	if err != nil {
		return genkind.Output{}, view{}, err
	}
	entry := manifest.Entry{
		Kind:      genkind.KindPort,
		Template:  genheader.Header{Kind: genkind.KindPort, Template: templateVersion}.Ref(),
		Name:      name,
		Contracts: []manifest.Contract{{Path: p.Path, Symbol: p.Name}},
		Owned:     []string{casesPath(p)},
	}
	for _, f := range files {
		entry.Regenerable = append(entry.Regenerable, f.Path)
	}
	return genkind.Output{Entry: entry, Regenerable: files}, v, nil
}

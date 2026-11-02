package adaptersql

import (
	"fmt"
	"go/format"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/holes"
	"github.com/cristianargotti/golaw/internal/gen/kinds/adapterkit"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/gen/render"
	"github.com/cristianargotti/golaw/internal/genheader"
)

// renderer renders the files of one plan from the embedded templates.
type renderer struct {
	p    *sqlPlan
	in   genkind.Input
	tmpl fs.FS
}

// The imports every adapter test file names, held once for the files.
const (
	sqlPath     = "database/sql"
	testingPath = "testing"
	errorsPath  = "errors"
)

func newRenderer(p *sqlPlan, in genkind.Input) (renderer, error) {
	tmpl, err := render.Templates(genkind.KindAdapterSQL, templateVersion)
	if err != nil {
		return renderer{}, fmt.Errorf(wrapped, p.target.Name, err)
	}
	return renderer{p: p, in: in, tmpl: tmpl}, nil
}

// gen renders one template of the kind over the plan's data.
func (r renderer) gen(name string, header genheader.Header, imports []genkind.Import) ([]byte, error) {
	// render.Go is held as a value because goast.goleak reads every call of
	// a selector named Go as a goroutine start, and this one starts none.
	renderGo := render.Go
	out, err := renderGo(r.tmpl, name, r.p.data, header, imports, r.in.Module)
	if err != nil {
		return nil, fmt.Errorf(wrapped, r.p.target.Name, err)
	}
	return out, nil
}

func (r renderer) header() genheader.Header {
	return genheader.Header{Version: r.in.Version, Kind: genkind.KindAdapterSQL, Template: templateVersion}
}

// file is the slash path of one file of the adapter's directory.
func (r renderer) file(suffix string) string { return r.p.target.Dir + "/" + suffix }

// fresh renders a new instance: the regenerable files, every owned file
// with its holes and red tests, the entry and the binding.
func (p *sqlPlan) fresh(in genkind.Input) (genkind.Output, error) {
	r, err := newRenderer(p, in)
	if err != nil {
		return genkind.Output{}, err
	}
	regenerable, err := r.regenerable()
	if err != nil {
		return genkind.Output{}, err
	}
	owned, err := r.owned()
	if err != nil {
		return genkind.Output{}, err
	}
	entry := manifest.Entry{
		Kind: genkind.KindAdapterSQL, Template: r.header().Ref(), Name: p.target.Name.String(),
		Contracts: p.contracts, Regenerable: paths(regenerable), Owned: paths(owned),
	}
	return genkind.Output{Entry: entry, Regenerable: regenerable, Owned: owned, Bindings: p.bindings(in, entry)}, nil
}

// bindings binds the adapter into the wiring when it is the first of its port.
func (p *sqlPlan) bindings(in genkind.Input, entry manifest.Entry) []genkind.Binding {
	t := p.target
	return adapterkit.Bindings(in.Manifest, entry, genkind.Binding{
		Kind: genkind.KindAdapterSQL, Area: t.Port.Package, Port: t.Port.Name, Import: t.Package,
		Call:  t.Name.Area + ".New" + t.Name.Type + "(infra.DB)",
		Needs: []genkind.InfraUse{{Field: "DB", Type: "*sql.DB", Import: genkind.Import{Path: sqlPath}}},
	})
}

// regenerable renders the store and its contract suite, sorted by path.
func (r renderer) regenerable() ([]genkind.File, error) {
	t := r.p.target
	store, err := r.gen("regenerable.go.tmpl", r.header(), []genkind.Import{{Path: sqlPath}, r.sqlc(), t.PortPkg})
	if err != nil {
		return nil, err
	}
	suite, err := r.gen("contract_test.go.tmpl", r.header(), []genkind.Import{{Path: testingPath}, t.Suite})
	if err != nil {
		return nil, err
	}
	lower := r.p.data.Lower
	return []genkind.File{
		{Path: r.file("zz_generated_" + lower + ".go"), Content: store},
		{Path: r.file("zz_generated_" + lower + "_contract_test.go"), Content: suite},
	}, nil
}

// owned renders the files the agent owns from then on, sorted by path.
func (r renderer) owned() ([]genkind.File, error) {
	out, err := r.ownedMain()
	if err != nil {
		return nil, err
	}
	if len(r.p.newMappers) > 0 {
		internal, internalErr := r.internalFile()
		if internalErr != nil {
			return nil, internalErr
		}
		out = append(out, internal)
	}
	if !r.p.pkg.Doc {
		doc, docErr := r.gen("doc.go.tmpl", genheader.Header{}, nil)
		if docErr != nil {
			return nil, docErr
		}
		out = append(out, genkind.File{Path: r.file("doc.go"), Content: doc})
	}
	slices.SortFunc(out, func(a, b genkind.File) int { return strings.Compare(a.Path, b.Path) })
	return out, nil
}

// ownedMain renders the main file, its red tests and the integration subject.
func (r renderer) ownedMain() ([]genkind.File, error) {
	d, lower := &r.p.data, r.p.data.Lower
	stubs, _, err := r.stubs(d.Methods, r.p.newMappers)
	if err != nil {
		return nil, err
	}
	reds, err := r.reds(d.Methods)
	if err != nil {
		return nil, err
	}
	specs := []struct {
		rel, tmpl string
		decls     [][]byte
		imports   []genkind.Import
	}{
		{lower + ".go", "owned.go.tmpl", stubs, r.mainImports(d.Methods, r.p.newMappers)},
		{lower + "_test.go", "owned_test.go.tmpl", reds, r.testImports(d.Methods)},
		{lower + "_integration_test.go", "integration_test.go.tmpl", nil, []genkind.Import{{Path: testingPath}, r.support(), r.p.target.Suite}},
	}
	out := make([]genkind.File, 0, len(specs))
	for _, s := range specs {
		f, fileErr := r.ownedFile(s.rel, s.tmpl, s.decls, s.imports)
		if fileErr != nil {
			return nil, fileErr
		}
		out = append(out, f)
	}
	return out, nil
}

// internalFile renders the internal test file with a red test per fresh mapper.
func (r renderer) internalFile() (genkind.File, error) {
	reds, err := r.mapperReds(r.p.newMappers)
	if err != nil {
		return genkind.File{}, err
	}
	return r.ownedFile(r.p.data.Lower+"_internal_test.go", "internal_test.go.tmpl", reds, r.internalImports())
}

// ownedFile renders an owned file without the golaw header, its stubs
// appended the way a later run appends, so both paths write one shape.
func (r renderer) ownedFile(rel, name string, decls [][]byte, imports []genkind.Import) (genkind.File, error) {
	full := r.file(rel)
	content, err := r.gen(name, genheader.Header{}, imports)
	if err != nil || len(decls) == 0 {
		return genkind.File{Path: full, Content: content}, err
	}
	content, err = holes.Append(content, genkind.Append{Path: full, Decls: decls})
	if err != nil {
		return genkind.File{}, fmt.Errorf("adapter-sql %s: %w", full, err)
	}
	content, err = format.Source(content)
	if err != nil {
		return genkind.File{}, fmt.Errorf("%w: %s: format: %w", render.ErrTemplate, full, err)
	}
	return genkind.File{Path: full, Content: content}, nil
}

func paths(files []genkind.File) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, f.Path)
	}
	slices.Sort(out)
	return out
}

// support is the shared support package, named explicitly when its
// directory is not gensupport, because every hole call spells gensupport.
func (r renderer) support() genkind.Import {
	imp := genkind.Import{Path: r.in.Module + "/" + r.in.Policy.Support}
	if path.Base(imp.Path) != holes.SupportName {
		imp.Name = holes.SupportName
	}
	return imp
}

func (r renderer) sqlc() genkind.Import {
	return genkind.Import{Path: r.in.Module + "/" + r.in.Policy.ThirdParty.SQLCOut}
}

package adapterhttp

import (
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/holes"
	"github.com/cristianargotti/golaw/internal/gen/kinds/adapterkit"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/gen/render"
	"github.com/cristianargotti/golaw/internal/genheader"
)

// renderGo renders one Go file. render.Go is held as a value because
// goast.goleak reads every call of a selector named Go as a goroutine start,
// and this one starts none.
func renderGo(tmpl fs.FS, name string, d any, header genheader.Header, imps []genkind.Import, module string) ([]byte, error) {
	goFile := render.Go
	out, err := goFile(tmpl, name, d, header, imps, module)
	if err != nil {
		return nil, fmt.Errorf(wrapped, err)
	}
	return out, nil
}

// files are the paths of one instance, from the directory and the type.
type files struct {
	main, test, doc, regenerable, suite string
}

func (p httpPlan) files() files {
	at := func(name string) string { return path.Join(p.target.Dir, name) }
	return files{
		main: at(p.data.Lower + ".go"), test: at(p.data.Lower + "_test.go"), doc: at("doc.go"),
		regenerable: at("zz_generated_" + p.data.Lower + ".go"), suite: at("zz_generated_" + p.data.Lower + "_contract_test.go"),
	}
}

func templates() (fs.FS, error) {
	tmpl, err := render.Templates(genkind.KindAdapterHTTP, templateVersion)
	if err != nil {
		return nil, fmt.Errorf(wrapped, err)
	}
	return tmpl, nil
}

func imports(paths ...string) []genkind.Import {
	out := make([]genkind.Import, 0, len(paths))
	for _, p := range paths {
		out = append(out, genkind.Import{Path: p})
	}
	return out
}

// pkgImport is the adapter's package as an external test imports it, named
// when the layout puts the package below a directory of another name.
func (p httpPlan) pkgImport() genkind.Import {
	imp := p.target.Package
	if path.Base(imp.Path) != p.data.Package {
		imp.Name = p.data.Package
	}
	return imp
}

// regenerable renders the two files the kind rewrites on every run.
func (p httpPlan) regenerable(in genkind.Input, tmpl fs.FS) ([]genkind.File, error) {
	header := genheader.Header{Version: in.Version, Kind: genkind.KindAdapterHTTP, Template: templateVersion}
	f := p.files()
	support := genkind.Import{Path: in.Module + "/" + in.Policy.Support}
	main, err := renderGo(tmpl, "regenerable.go.tmpl", p.data, header,
		append(imports("net/http"), support, p.target.PortPkg), in.Module)
	if err != nil {
		return nil, err
	}
	suite, err := renderGo(tmpl, "contract_test.go.tmpl", p.data, header,
		append(imports("testing"), p.target.Suite, p.pkgImport()), in.Module)
	if err != nil {
		return nil, err
	}
	return []genkind.File{{Path: f.regenerable, Content: main}, {Path: f.suite, Content: suite}}, nil
}

// stubs renders the method of each hole and answers the imports its types
// and the hole call need.
func (p httpPlan) stubs(in genkind.Input, tmpl fs.FS, methods []method) (genkind.Append, error) {
	add := genkind.Append{Path: p.files().main, Imports: []genkind.Import{{Path: in.Module + "/" + in.Policy.Support}}}
	var types []string
	for i := range methods {
		decl, err := adapterkit.Decl(tmpl, "method.go.tmpl", stub{D: p.data, M: methods[i]})
		if err != nil {
			return genkind.Append{}, fmt.Errorf(wrapped, err)
		}
		add.Decls = append(add.Decls, decl)
		add.Holes = append(add.Holes, methods[i].Hole)
		types = append(types, methods[i].Signature())
	}
	add.Imports = append(add.Imports, adapterkit.Uses(p.imports, types)...)
	return add, nil
}

// reds renders the red test of each hole, every one over a TLS test server
// answering 404, so a fill is exercised without a network.
func (p httpPlan) reds(in genkind.Input, tmpl fs.FS, methods []method) (genkind.Append, error) {
	support := in.Module + "/" + in.Policy.Support
	add := genkind.Append{Path: p.files().test, Imports: append(imports("errors", "net/http", "testing", support, support+"/tlstest"), p.pkgImport())}
	var types []string
	for i := range methods {
		decl, err := adapterkit.Decl(tmpl, "red_test.go.tmpl", stub{D: p.data, M: methods[i]})
		if err != nil {
			return genkind.Append{}, fmt.Errorf(wrapped, err)
		}
		add.Decls = append(add.Decls, decl)
		for _, input := range methods[i].Inputs {
			types = append(types, input.Type)
		}
	}
	add.Imports = append(add.Imports, adapterkit.Uses(p.imports, types)...)
	return add, nil
}

// owned renders an owned file as its package clause plus the appended
// declarations, so a new file and a later append share one rendering.
func owned(in genkind.Input, tmpl fs.FS, name string, d data, add genkind.Append) (genkind.File, error) {
	clause, err := renderGo(tmpl, name, d, genheader.Header{}, nil, in.Module)
	if err != nil {
		return genkind.File{}, err
	}
	content, err := holes.Append(clause, add)
	if err != nil {
		return genkind.File{}, fmt.Errorf(wrapped, err)
	}
	return genkind.File{Path: add.Path, Content: content}, nil
}

// fresh renders a new instance: the regenerable files, the owned main and
// test files with every hole open, and doc.go when the package has none.
func (p httpPlan) fresh(in genkind.Input) (genkind.Output, error) {
	tmpl, err := templates()
	if err != nil {
		return genkind.Output{}, err
	}
	regen, err := p.regenerable(in, tmpl)
	if err != nil {
		return genkind.Output{}, err
	}
	ownedFiles, err := p.ownedFiles(in, tmpl)
	if err != nil {
		return genkind.Output{}, err
	}
	var ownedPaths []string
	for _, f := range ownedFiles {
		ownedPaths = append(ownedPaths, f.Path)
	}
	entry := p.entry(regen, ownedPaths)
	return genkind.Output{Entry: entry, Regenerable: regen, Owned: ownedFiles, Bindings: p.bindings(in, entry)}, nil
}

// ownedFiles renders the owned files of a new instance, sorted by path.
func (p httpPlan) ownedFiles(in genkind.Input, tmpl fs.FS) ([]genkind.File, error) {
	stubs, err := p.stubs(in, tmpl, p.data.Methods)
	if err != nil {
		return nil, err
	}
	main, err := owned(in, tmpl, "owned.go.tmpl", p.data, stubs)
	if err != nil {
		return nil, err
	}
	reds, err := p.reds(in, tmpl, p.data.Methods)
	if err != nil {
		return nil, err
	}
	seed, err := adapterkit.Decl(tmpl, "seed_test.go.tmpl", p.data)
	if err != nil {
		return nil, fmt.Errorf(wrapped, err)
	}
	reds.Decls = append(reds.Decls, seed)
	reds.Imports = append(reds.Imports, p.target.Suite)
	test, err := owned(in, tmpl, "owned_test.go.tmpl", p.data, reds)
	if err != nil {
		return nil, err
	}
	out := []genkind.File{main, test}
	if !p.pkg.Doc {
		doc, docErr := renderGo(tmpl, "doc.go.tmpl", p.data, genheader.Header{}, nil, in.Module)
		if docErr != nil {
			return nil, docErr
		}
		out = append(out, genkind.File{Path: p.files().doc, Content: doc})
	}
	slices.SortFunc(out, func(a, b genkind.File) int { return strings.Compare(a.Path, b.Path) })
	return out, nil
}

func (p httpPlan) entry(regen []genkind.File, ownedPaths []string) manifest.Entry {
	regenPaths := make([]string, 0, len(regen))
	for _, f := range regen {
		regenPaths = append(regenPaths, f.Path)
	}
	return manifest.Entry{
		Kind: genkind.KindAdapterHTTP, Template: genheader.Header{Kind: genkind.KindAdapterHTTP, Template: templateVersion}.Ref(),
		Name: p.target.Name.String(), Contracts: []manifest.Contract{p.target.Contract},
		Regenerable: slices.Sorted(slices.Values(regenPaths)), Owned: slices.Sorted(slices.Values(ownedPaths)),
	}
}

// bindings binds the adapter into the wiring over one configuration field
// of the infra value, named by package and type so two clients never share it.
func (p httpPlan) bindings(in genkind.Input, entry manifest.Entry) []genkind.Binding {
	first, size := utf8.DecodeRuneInString(p.data.Package)
	field := string(unicode.ToUpper(first)) + p.data.Package[size:] + p.data.Type
	pkg := p.pkgImport()
	return adapterkit.Bindings(in.Manifest, entry, genkind.Binding{
		Kind: genkind.KindAdapterHTTP, Area: p.target.Port.Package, Port: p.target.Port.Name, Import: pkg,
		Call:  p.data.Package + ".New" + p.data.Type + "(infra." + field + ")",
		Needs: []genkind.InfraUse{{Field: field, Type: p.data.Package + "." + p.data.Type + "Config", Import: pkg}},
	})
}

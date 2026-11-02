package port

import (
	"fmt"
	"path"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/holes"
	"github.com/cristianargotti/golaw/internal/gen/render"
	"github.com/cristianargotti/golaw/internal/genheader"
)

// templateVersion is the version of templates/gen/port this kit embeds; a
// header names it so a later kit tells its own output from an older one.
const templateVersion = 1

// The regenerable files of a port test package.
const (
	fakeFile      = "zz_generated_fake.go"
	suiteFile     = "zz_generated_suite.go"
	suiteTestFile = "zz_generated_suite_test.go"
	porttestSub   = "/porttest"
	slicesPkg     = "slices"
)

// testDir is the port's test package beside the consuming package (M7.5).
func testDir(p genkind.Port) string {
	return p.Dir + "/" + strings.ToLower(p.Name) + "test"
}

// supportImport is a package of the support tree. The root is imported as
// gensupport wherever the policy moved it, since the hole finder and the
// templates recognize the support calls by that name.
func supportImport(in genkind.Input, sub string) genkind.Import {
	imp := genkind.Import{Path: in.Module + "/" + in.Policy.Support + sub}
	if sub == "" && path.Base(in.Policy.Support) != holes.SupportName {
		imp.Name = holes.SupportName
	}
	return imp
}

// renderFile renders one template of the port kind; an owned file passes
// header false, because only a regenerable file carries the golaw header.
func renderFile(in genkind.Input, name string, v view, header bool, imports []genkind.Import) ([]byte, error) {
	fsys, err := render.Templates(genkind.KindPort, templateVersion)
	if err != nil {
		return nil, fmt.Errorf("port %s.%s: %w", v.Area, v.Port, err)
	}
	var h genheader.Header
	if header {
		h = genheader.Header{Version: in.Version, Kind: genkind.KindPort, Template: templateVersion}
	}
	// render.Go is held as a value because goast.goleak reads every call of
	// a selector named Go as a goroutine start, and this one starts none.
	renderGo := render.Go
	out, err := renderGo(fsys, name+".tmpl", v, h, imports, in.Module)
	if err != nil {
		return nil, fmt.Errorf("port %s.%s: %w", v.Area, v.Port, err)
	}
	return out, nil
}

// regenerable renders the fake, the suite runner and the suite test, each
// with the imports its types name and nothing more, so no rendering needs
// goimports to build.
func regenerable(in genkind.Input, p genkind.Port, v view) ([]genkind.File, error) {
	sigs, err := signatures(p)
	if err != nil {
		return nil, err
	}
	resolvers := []struct {
		name    string
		imports func(genkind.Input, genkind.Port, view, []signature) ([]genkind.Import, error)
	}{{fakeFile, fakeImports}, {suiteFile, suiteImports}, {suiteTestFile, suiteTestImports}}
	files := make([]genkind.File, 0, len(resolvers))
	for _, r := range resolvers {
		imports, err := r.imports(in, p, v, sigs)
		if err != nil {
			return nil, err
		}
		content, err := renderFile(in, r.name, v, true, imports)
		if err != nil {
			return nil, err
		}
		files = append(files, genkind.File{Path: testDir(p) + "/" + r.name, Content: content})
	}
	return files, nil
}

// fakeImports names every type of every method and the support root that
// holds the recorder; porttest joins once a method answers through it.
func fakeImports(in genkind.Input, p genkind.Port, _ view, sigs []signature) ([]genkind.Import, error) {
	names := []string{p.Package}
	var fixed []string
	for _, s := range sigs {
		names = append(names, s.packages()...)
	}
	if len(sigs) > 0 {
		fixed = append(fixed, supportImport(in, porttestSub).Path)
	}
	imports, err := importsFor(p, in.Module, names, fixed...)
	if err != nil {
		return nil, err
	}
	return append(imports, supportImport(in, "")), nil
}

// suiteImports names the parameter types alone: the call functions read
// the arguments as their types and hand every result back as any.
func suiteImports(in genkind.Input, p genkind.Port, v view, sigs []signature) ([]genkind.Import, error) {
	names := []string{p.Package}
	fixed := []string{"testing", supportImport(in, porttestSub).Path}
	for _, s := range sigs {
		names = append(names, s.paramPackages()...)
	}
	if len(sigs) > 0 {
		fixed = append(fixed, "context")
	}
	if v.Concat {
		fixed = append(fixed, slicesPkg)
	}
	return importsFor(p, in.Module, names, fixed...)
}

// suiteTestImports names the types of the variables the recording test
// declares for the first method.
func suiteTestImports(in genkind.Input, p genkind.Port, v view, sigs []signature) ([]genkind.Import, error) {
	var names []string
	fixed := []string{slicesPkg, "testing", in.Module + "/" + testDir(p)}
	if v.First != nil {
		names, fixed = sigs[0].paramPackages(), append(fixed, "errors")
	}
	return importsFor(p, in.Module, names, fixed...)
}

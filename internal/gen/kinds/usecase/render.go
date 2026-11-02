package usecase

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/holes"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/gen/render"
	"github.com/cristianargotti/golaw/internal/genheader"
)

// templateVersion is the version of templates/gen/usecase this kit embeds.
const templateVersion = 1

// The area files every use case entry of an area renders byte identically.
const (
	areaFile     = "zz_generated_usecases.go"
	areaTestFile = "zz_generated_usecases_test.go"
)

// piece is one file of a use case: its template, its imports and whether
// it is regenerable, which alone carries the golaw header.
type piece struct {
	path, template string
	imports        []genkind.Import
	regenerable    bool
}

// files renders the five files of e and the binding the wiring kind reads.
func files(in genkind.Input, dir string, v view, e manifest.Entry) (genkind.Output, error) {
	fsys, err := render.Templates(genkind.KindUsecase, templateVersion)
	if err != nil {
		return genkind.Output{}, fmt.Errorf("usecase %s: %w", e.Name, err)
	}
	area := areaImport(in, dir, v.Package)
	out := genkind.Output{Entry: e, Bindings: []genkind.Binding{{
		Kind: genkind.KindUsecase, Area: e.Area(), Import: area, Call: v.Package + ".NewUseCases",
	}}}
	// render.Go is held as a value because goast.goleak reads every call of
	// a selector named Go as a goroutine start, and this one starts none.
	renderGo := render.Go
	for _, p := range pieces(in, dir, v, e, area) {
		var h genheader.Header
		if p.regenerable {
			h = genheader.Header{Version: in.Version, Kind: genkind.KindUsecase, Template: templateVersion}
		}
		content, err := renderGo(fsys, p.template, v, h, p.imports, in.Module)
		if err != nil {
			return genkind.Output{}, fmt.Errorf("usecase %s: %w", e.Name, err)
		}
		file := genkind.File{Path: p.path, Content: content}
		if p.regenerable {
			out.Regenerable = append(out.Regenerable, file)
		} else {
			out.Owned = append(out.Owned, file)
		}
	}
	return out, nil
}

// pieces lists the files with the imports each one names and nothing more,
// the regenerable ones in path order as the entry lists them.
func pieces(in genkind.Input, dir string, v view, e manifest.Entry, area genkind.Import) []piece {
	support := supportImport(in, "")
	std := func(paths ...string) []genkind.Import {
		out := make([]genkind.Import, 0, len(paths))
		for _, p := range paths {
			out = append(out, genkind.Import{Path: p})
		}
		return out
	}
	areaTest := slices.Concat(std("context", "errors", "testing"), []genkind.Import{area, support, supportImport(in, "/porttest")})
	for _, port := range v.Ports {
		areaTest = append(areaTest, genkind.Import{Path: in.Module + "/" + dir + "/" + port.Package})
	}
	stem := dir + "/" + snake(e.Type())
	out := make([]piece, 0, len(e.Regenerable)+len(e.Owned))
	out = append(out,
		piece{path: dir + "/zz_generated_" + snake(e.Type()) + ".go", template: "zz_generated_usecase.go.tmpl", imports: []genkind.Import{support}, regenerable: true},
		piece{path: dir + "/" + areaFile, template: areaFile + ".tmpl", regenerable: true},
		piece{path: dir + "/" + areaTestFile, template: areaTestFile + ".tmpl", imports: areaTest, regenerable: true},
	)
	slices.SortFunc(out, func(a, b piece) int { return strings.Compare(a.path, b.path) })
	return append(out,
		piece{path: stem + ".go", template: "usecase.go.tmpl", imports: slices.Concat(std("context"), []genkind.Import{support})},
		piece{path: stem + "_test.go", template: "usecase_test.go.tmpl", imports: slices.Concat(std("errors", "testing"), []genkind.Import{area, support})},
	)
}

// areaImport is the consuming package, named when its clause differs from
// its directory so the test package and the wiring call it by its clause.
func areaImport(in genkind.Input, dir, pkg string) genkind.Import {
	imp := genkind.Import{Path: in.Module + "/" + dir}
	if path.Base(dir) != pkg {
		imp.Name = pkg
	}
	return imp
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

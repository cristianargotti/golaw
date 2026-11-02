package wiring

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/holes"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/gen/render"
	"github.com/cristianargotti/golaw/internal/genheader"
	"github.com/cristianargotti/golaw/internal/model"
)

// templateVersion is the template tree this kit embeds for the kind.
const templateVersion = 1

// constraint is what the agent fills the one hole of the root with: the
// values only the deployment knows, so the rest of the root stays generated.
const constraint = "open every value Infra holds from the configuration (pool, clients), " +
	"serve Wire(infra) through an http.Server that sets ReadHeaderTimeout, " +
	"close them when ctx ends and return nil once ctx is done"

// The files of the root, by name inside the main package directory.
const (
	wireFile     = "zz_generated_wiring.go"
	wireTestFile = "zz_generated_wiring_internal_test.go"
	mainFile     = "main.go"
	serveFile    = "wiring.go"
	serveTest    = "wiring_internal_test.go"
)

// rootMain names the entry of the root layout, whose package is main.
const rootMain = "main"

type kind struct{}

// Kind answers the wiring kind.
func Kind() genkind.Wirer { return kind{} }

func (kind) Name() string { return genkind.KindWiring }

func (kind) Template() int { return templateVersion }

// New refuses: no command names a wiring instance, the engine renders it.
func (kind) New(context.Context, genkind.Input, genkind.Request) (genkind.Output, error) {
	return genkind.Output{}, fmt.Errorf("%w: the wiring is rendered by Wire from the bindings of every entry", genkind.ErrRequest)
}

// Regenerate refuses for the reason New does: the entry alone does not hold
// the bindings the root is rendered from.
func (k kind) Regenerate(ctx context.Context, in genkind.Input, _ manifest.Entry) (genkind.Output, error) {
	return k.New(ctx, in, genkind.Request{})
}

// owned is what the owned templates read.
type owned struct {
	ID, Constraint, SupportName, Main string
}

// spec is one file of the root to render.
type spec struct {
	name    string
	data    any
	imports []genkind.Import
}

// Wire renders the root of main ("" for the root layout) from the bindings.
func (kind) Wire(ctx context.Context, in genkind.Input, main string, bindings []genkind.Binding) (genkind.Output, error) {
	if err := ctx.Err(); err != nil {
		return genkind.Output{}, err
	}
	dir := wiringDir(in.Policy, main)
	comp, decisions, err := compose(bindings, path.Join(dir, wireFile))
	if err != nil {
		return genkind.Output{}, err
	}
	templates, err := render.Templates(genkind.KindWiring, templateVersion)
	if err != nil {
		return genkind.Output{}, fmt.Errorf("wiring: %w", err)
	}
	header := genheader.Header{Version: in.Version, Kind: genkind.KindWiring, Template: templateVersion}
	name := main
	if name == "" {
		name = rootMain
	}
	out := genkind.Output{Decisions: decisions, Entry: manifest.Entry{Kind: genkind.KindWiring, Template: header.Ref(), Name: name + ".Wire"}}
	out.Regenerable, err = renderAll(templates, in.Module, dir, header, regenerable(comp))
	if err != nil {
		return genkind.Output{}, err
	}
	root, err := readRoot(in.Snapshot, dir)
	if err != nil {
		return genkind.Output{}, err
	}
	withMain := !root.main
	out.Owned, err = renderAll(templates, in.Module, dir, genheader.Header{}, ownedSpecs(in, main, withMain))
	if err != nil {
		return genkind.Output{}, err
	}
	out.Entry.Regenerable = paths(out.Regenerable)
	out.Entry.Owned = paths(out.Owned)
	if !withMain && in.Snapshot.Class(path.Join(dir, mainFile)) != model.ClassUnknown {
		out.Entry.Owned = append([]string{path.Join(dir, mainFile)}, out.Entry.Owned...)
	}
	return out, nil
}

// wiringDir places the root: the module root for the root layout, else the
// layout's wiring pattern for main.
func wiringDir(policy model.GeneratePolicy, main string) string {
	if main == "" {
		return "."
	}
	return strings.ReplaceAll(policy.Layout.Wiring, "{main}", main)
}

// stdContext is the import path every Go file of the root needs.
const stdContext = "context"

// runImports are the imports Run adds to the composition's own.
func runImports() []genkind.Import {
	return []genkind.Import{{Path: stdContext}, {Path: "log/slog"}, {Path: "os"}, {Path: "os/signal"}}
}

func regenerable(comp composition) []spec {
	imports := slices.Clone(comp.Imports)
	for _, imp := range runImports() {
		if !slices.Contains(imports, imp) {
			imports = append(imports, imp)
		}
	}
	return []spec{
		{name: wireFile, data: comp, imports: imports},
		{name: wireTestFile, data: comp, imports: []genkind.Import{{Path: stdContext}, {Path: "errors"}, {Path: "testing"}}},
	}
}

// ownedSpecs lists main.go first only when no file of the package declares
// main, because a second main function would not compile.
func ownedSpecs(in genkind.Input, main string, withMain bool) []spec {
	support := genkind.Import{Path: in.Module + "/" + in.Policy.Support}
	if path.Base(in.Policy.Support) != holes.SupportName {
		support.Name = holes.SupportName
	}
	command := main
	if command == "" {
		command = path.Base(in.Module)
	}
	data := owned{ID: holes.ID("wiring", "Infra"), Constraint: constraint, SupportName: holes.SupportName, Main: command}
	ctx := genkind.Import{Path: stdContext}
	var out []spec
	if withMain {
		out = append(out, spec{name: mainFile, data: data, imports: []genkind.Import{ctx, {Path: "os"}}})
	}
	return append(out,
		spec{name: serveFile, data: data, imports: []genkind.Import{ctx, {Path: "fmt"}, support}},
		spec{name: serveTest, data: data, imports: []genkind.Import{ctx, {Path: "errors"}, {Path: "testing"}, support}},
	)
}

func renderAll(templates fs.FS, module, dir string, header genheader.Header, specs []spec) ([]genkind.File, error) {
	// render.Go is held as a value because goast.goleak reads every call of
	// a selector named Go as a goroutine start, and this one starts none.
	renderGo := render.Go
	files := make([]genkind.File, 0, len(specs))
	for _, s := range specs {
		content, err := renderGo(templates, s.name+".tmpl", s.data, header, s.imports, module)
		if err != nil {
			return nil, fmt.Errorf("wiring: %w", err)
		}
		files = append(files, genkind.File{Path: path.Join(dir, s.name), Content: content})
	}
	return files, nil
}

func paths(files []genkind.File) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, f.Path)
	}
	return out
}

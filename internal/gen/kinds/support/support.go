package support

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/gen/render"
	"github.com/cristianargotti/golaw/internal/genheader"
)

// templateVersion is the template tree this kit embeds for the kind.
const templateVersion = 1

// templateSuffix marks a file of the tree as a template to render.
const templateSuffix = ".tmpl"

type kind struct{}

// Kind answers the support kind.
func Kind() genkind.Kind { return kind{} }

func (kind) Name() string { return genkind.KindSupport }

func (kind) Template() int { return templateVersion }

// New renders the support package; it takes no request because the package
// is the same for every repository but its path and module.
func (k kind) New(ctx context.Context, in genkind.Input, _ genkind.Request) (genkind.Output, error) {
	return k.render(ctx, in)
}

// Regenerate renders the same bytes as New: the package holds no state an
// entry would carry, so a regenerated package never differs from a new one.
func (k kind) Regenerate(ctx context.Context, in genkind.Input, _ manifest.Entry) (genkind.Output, error) {
	return k.render(ctx, in)
}

// data is what every support template reads.
type data struct {
	Package string // the package clause of the file
	Import  string // the import path of the file's package
	Support string // the import path of the support root
	Module  string
}

func (kind) render(ctx context.Context, in genkind.Input) (genkind.Output, error) {
	if err := ctx.Err(); err != nil {
		return genkind.Output{}, err
	}
	if in.Module == "" || in.Policy.Support == "" {
		return genkind.Output{}, fmt.Errorf("%w: the support package needs the module path and the support path", genkind.ErrRequest)
	}
	templates, err := render.Templates(genkind.KindSupport, templateVersion)
	if err != nil {
		return genkind.Output{}, fmt.Errorf("support: %w", err)
	}
	names, err := walk(templates)
	if err != nil {
		return genkind.Output{}, err
	}
	header := genheader.Header{Version: in.Version, Kind: genkind.KindSupport, Template: templateVersion}
	out := genkind.Output{Entry: manifest.Entry{
		Kind: genkind.KindSupport, Template: header.Ref(), Name: path.Base(in.Policy.Support) + ".Support",
	}}
	// render.Go is held as a value because goast.goleak reads every call of
	// a selector named Go as a goroutine start, and this one starts none.
	renderGo := render.Go
	for _, name := range names {
		target := path.Join(in.Policy.Support, strings.TrimSuffix(name, templateSuffix))
		dir := path.Dir(target)
		d := data{Package: path.Base(dir), Import: in.Module + "/" + dir, Support: in.Module + "/" + in.Policy.Support, Module: in.Module}
		content, err := renderGo(templates, name, d, header, nil, in.Module)
		if err != nil {
			return genkind.Output{}, fmt.Errorf("support: %w", err)
		}
		out.Regenerable = append(out.Regenerable, genkind.File{Path: target, Content: content})
		out.Entry.Regenerable = append(out.Entry.Regenerable, target)
	}
	return out, nil
}

// walk lists every template of the tree in lexical order, so a template a
// later kind adds to the support tree needs no edit here.
func walk(templates fs.FS) ([]string, error) {
	var names []string
	err := fs.WalkDir(templates, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(name, templateSuffix) {
			names = append(names, name)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("%w: support templates: %w", render.ErrTemplate, err)
	}
	return names, nil
}

package adapterkit

import (
	"bytes"
	"fmt"
	"go/format"
	"io/fs"
	"path"
	"text/template"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/gen/render"
)

// clause makes one declaration a file go/format accepts; it is cut again
// from the result, so the caller gets the declaration alone.
const clause = "package p\n\n"

// failed is the text of every refusal of Decl, the template named first.
const failed = "%w: %s: %w"

// Decl renders one top-level declaration for an append to owned code: the
// owned file keeps its own imports and header, so only the declaration is
// rendered and formatted.
func Decl(templates fs.FS, name string, data any) ([]byte, error) {
	tmpl, err := template.New(path.Base(name)).Funcs(render.Funcs()).Option("missingkey=error").ParseFS(templates, name)
	if err != nil {
		return nil, fmt.Errorf(failed, render.ErrTemplate, name, err)
	}
	var buf bytes.Buffer
	if execErr := tmpl.Execute(&buf, data); execErr != nil {
		return nil, fmt.Errorf(failed, render.ErrTemplate, name, execErr)
	}
	out, err := format.Source(append([]byte(clause), buf.Bytes()...))
	if err != nil {
		return nil, fmt.Errorf(failed, render.ErrTemplate, name, err)
	}
	return bytes.TrimPrefix(out, []byte(clause)), nil
}

// Bindings answers b only when entry is the first adapter of its port in
// manifest order: a second adapter would bind one Deps field twice, so the
// person wires another by hand.
func Bindings(m manifest.Manifest, entry manifest.Entry, b genkind.Binding) []genkind.Binding {
	if len(entry.Contracts) == 0 {
		return nil
	}
	entries := m.With(entry).Entries
	for i := range entries {
		e := &entries[i]
		if e.Kind != genkind.KindAdapterSQL && e.Kind != genkind.KindAdapterHTTP {
			continue
		}
		if len(e.Contracts) == 0 || e.Contracts[0] != entry.Contracts[0] {
			continue
		}
		if e.Kind == entry.Kind && e.Name == entry.Name {
			return []genkind.Binding{b}
		}
		return nil
	}
	return nil
}

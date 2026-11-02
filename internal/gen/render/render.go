package render

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"path"
	"slices"
	"text/template"

	"github.com/cristianargotti/golaw"
	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/genheader"
)

// failed spells every refusal of one template alike, so a caller reads the
// template name at the same place of each message.
const failed = "%w: %s: %w"

// unresolved is what text/template prints for a nil value; a rendering that
// holds it would compile into a wrong file, so it is refused.
const unresolved = "<no value>"

// Templates is the embedded tree of one kind and template version.
func Templates(kind string, version int) (fs.FS, error) {
	if !kindName(kind) || version < 1 {
		return nil, fmt.Errorf("%w: %q@%d is no kind template", ErrTemplate, kind, version)
	}
	dir := fmt.Sprintf("%s%s/%d", assets.GenPrefix, kind, version)
	if _, err := fs.Stat(golaw.FS, dir); err != nil {
		return nil, fmt.Errorf("%w: %s@%d not embedded: %w", ErrTemplate, kind, version, err)
	}
	sub, err := fs.Sub(golaw.FS, dir)
	if err != nil {
		return nil, fmt.Errorf(failed, ErrTemplate, dir, err)
	}
	return sub, nil
}

// kindName admits lower case words joined by hyphens, the spelling of a kind.
func kindName(kind string) bool {
	if kind == "" {
		return false
	}
	for _, c := range []byte(kind) {
		if (c < 'a' || c > 'z') && c != '-' {
			return false
		}
	}
	return true
}

// Go renders one template over data and answers formatted Go with the
// header, when header.Kind is set, and the complete import block.
func Go(templates fs.FS, name string, data any, header genheader.Header, imports []genkind.Import, module string) ([]byte, error) {
	out, err := execute(templates, name, data)
	if err != nil {
		return nil, err
	}
	out, err = assemble(out, name, imports, module)
	if err != nil {
		return nil, err
	}
	return finish(out, name, header)
}

// execute runs the template with a missing key as an error, because a
// silent empty value would render a file that compiles and is wrong.
func execute(templates fs.FS, name string, data any) ([]byte, error) {
	tmpl, err := template.New(path.Base(name)).Funcs(Funcs()).Option("missingkey=error").ParseFS(templates, name)
	if err != nil {
		return nil, fmt.Errorf(failed, ErrTemplate, name, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf(failed, ErrTemplate, name, err)
	}
	if bytes.Contains(buf.Bytes(), []byte(unresolved)) {
		return nil, fmt.Errorf("%w: %s: unresolved value", ErrTemplate, name)
	}
	return buf.Bytes(), nil
}

// assemble replaces every import declaration the template wrote with the
// one block the engine builds, so the grouping never depends on a template.
func assemble(src []byte, name string, imports []genkind.Import, module string) ([]byte, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: format: %w", ErrTemplate, name, err)
	}
	importBlock, err := block(file, imports, module)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	tf := fset.File(file.Pos())
	out := src
	for _, decl := range slices.Backward(file.Decls) {
		if gen, ok := decl.(*ast.GenDecl); ok && gen.Tok == token.IMPORT {
			out = slices.Concat(out[:tf.Offset(gen.Pos())], out[tf.Offset(gen.End()):])
		}
	}
	if importBlock == "" {
		return out, nil
	}
	at := tf.Offset(file.Name.End())
	return slices.Concat(out[:at], []byte("\n\n"+importBlock), out[at:]), nil
}

// finish puts the header on the first line and formats the whole file, so
// two renderings of one input are the same bytes.
func finish(src []byte, name string, header genheader.Header) ([]byte, error) {
	if header.Kind != "" {
		src = slices.Concat([]byte(header.Line()+"\n\n"), src)
	}
	out, err := format.Source(src)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: format: %w", ErrTemplate, name, err)
	}
	return out, nil
}

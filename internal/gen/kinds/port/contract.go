package port

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"path"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/model"
)

// portsFile is where a port the area does not declare yet gets its stub.
const portsFile = "ports.go"

// contractFile is where a port is declared in its area, or would be.
type contractFile struct {
	Path, Package string
	Declared      bool // a type spec of the port's name exists
	Exists        bool // Path is in the snapshot
	Undocumented  bool // no Go file of the directory carries a package comment yet
	empty         []string
}

// locate reads the area's Go files for the port's declaration. Every other
// interface without methods is kept aside, because a second one would be
// flagged by the lint asset's iface identical before its methods exist.
func locate(s model.Snapshot, dir, area, name string) (contractFile, error) {
	c := contractFile{Package: area, Path: dir + "/" + portsFile}
	files := model.Select(s, func(p string) bool {
		return path.Dir(p) == dir && strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go")
	})
	c.Undocumented = true
	fset := token.NewFileSet()
	for i := range files {
		f := &files[i]
		c.Exists = c.Exists || f.Path == c.Path
		content, err := s.Read(f.Path)
		if err != nil {
			return contractFile{}, fmt.Errorf("read %s for port %s.%s: %w", f.Path, area, name, err)
		}
		file, err := parser.ParseFile(fset, f.Path, content, parser.SkipObjectResolution|parser.ParseComments)
		if err != nil {
			return contractFile{}, fmt.Errorf("%w: %s does not parse: %w", genkind.ErrCannotJudge, f.Path, err)
		}
		c.Package, c.Undocumented = file.Name.Name, c.Undocumented && file.Doc == nil
		c.read(file, f.Path, name)
	}
	if c.Declared {
		c.Exists = true
	}
	return c, nil
}

// read records the declaration of name and every interface without methods.
func (c *contractFile) read(file *ast.File, filePath, name string) {
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			if ts, ok := spec.(*ast.TypeSpec); ok {
				c.readType(ts, filePath, name)
			}
		}
	}
}

// readType records one type spec of the area.
func (c *contractFile) readType(ts *ast.TypeSpec, filePath, name string) {
	if ts.Name.Name == name {
		c.Declared, c.Path = true, filePath
		return
	}
	if iface, ok := ts.Type.(*ast.InterfaceType); ok && len(iface.Methods.List) == 0 {
		c.empty = append(c.empty, ts.Name.Name)
	}
}

// twin refuses a port whose fake and suite would repeat, byte for byte,
// those of a port of the same name in another area: both depend only on
// the name, the methods and the test package name, and the copy would
// count as duplicated lines (M7.10.1).
func twin(in genkind.Input, req genkind.Request, p genkind.Port) error {
	for i := range in.Manifest.Entries {
		e := &in.Manifest.Entries[i]
		if e.Kind != genkind.KindPort || e.Type() != req.Name.Type || e.Area() == req.Name.Area || len(e.Contracts) == 0 {
			continue
		}
		// An entry the reader cannot read is no certain copy, so it never refuses.
		other, err := in.Contracts.Port(e.Contracts[0].Path, e.Contracts[0].Symbol)
		if err != nil || !sameMethods(p, other) {
			continue
		}
		return fmt.Errorf("%w: %s has the methods of %s, so its fake and contract suite would repeat those of %s byte for byte and count as duplicated lines: rename the port so its generated files differ, or keep it in %s",
			genkind.ErrRequest, req.Name, e.Name, e.Name, e.Area())
	}
	return nil
}

// sameMethods reports whether two ports would render the same fake: the
// same methods in the same order over types that name no area type.
func sameMethods(a, b genkind.Port) bool {
	if len(a.Methods) == 0 || len(a.Methods) != len(b.Methods) {
		return false
	}
	for i, m := range a.Methods {
		o := b.Methods[i]
		if m.Name != o.Name || !sameParams(m.Params, o.Params) || !sameParams(m.Results, o.Results) {
			return false
		}
	}
	return true
}

// sameParams compares the types alone, since the fake renames parameters.
func sameParams(a, b []genkind.Param) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Type != b[i].Type || namesAreaType(a[i].Type) {
			return false
		}
	}
	return true
}

// namesAreaType reports whether a type names an exported identifier no
// selector qualifies: an area type the fake qualifies with its own
// package, so the two renderings differ.
func namesAreaType(expr string) bool {
	node, err := parser.ParseExpr(strings.TrimPrefix(expr, variadic))
	if err != nil {
		return true
	}
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		switch e := n.(type) {
		case *ast.SelectorExpr:
			return false
		case *ast.Field:
			found = found || namesAreaType(types.ExprString(e.Type))
			return false
		case *ast.Ident:
			found = found || (e.IsExported() && types.Universe.Lookup(e.Name) == nil)
		}
		return true
	})
	return found
}

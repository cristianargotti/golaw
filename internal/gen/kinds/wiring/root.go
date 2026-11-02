package wiring

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/model"
)

// rootNames are the top level names the root golaw writes declares, each
// with the file of the root that declares it.
func rootNames() map[string]string {
	return map[string]string{"Run": wireFile, "Wire": wireFile, "Infra": wireFile, "Serve": serveFile}
}

// mainPackage is what the source files of the main package already declare.
type mainPackage struct {
	main bool // a func main without a receiver
}

// readRoot reads the tracked and untracked source files directly in dir:
// whether one declares func main, and a refusal naming the file when one
// other than the root's own declares a name the root declares, because the
// package would stop compiling once the root is written. A file that does
// not parse proves nothing, so it neither keeps main.go out nor refuses.
func readRoot(s model.Snapshot, dir string) (mainPackage, error) {
	var pkg mainPackage
	names := rootNames()
	for _, f := range s.Files() {
		if (f.Class != model.ClassTracked && f.Class != model.ClassUntracked) ||
			path.Dir(f.Path) != dir || !strings.HasSuffix(f.Path, ".go") || strings.HasSuffix(f.Path, "_test.go") {
			continue
		}
		for _, name := range declaredIn(s, f.Path) {
			pkg.main = pkg.main || name == rootMain
			if owner, ok := names[name]; ok && path.Base(f.Path) != owner {
				return mainPackage{}, fmt.Errorf("%w: %s declares %s, which golaw writes in %s of the composition root: rename it so the package keeps compiling",
					genkind.ErrRequest, f.Path, name, path.Join(dir, owner))
			}
		}
	}
	return pkg, nil
}

// declaredIn answers the names a source file declares, none when it cannot
// be read or does not parse.
func declaredIn(s model.Snapshot, rel string) []string {
	content, err := s.Read(rel)
	if err != nil {
		return nil
	}
	file, err := parser.ParseFile(token.NewFileSet(), rel, content, parser.SkipObjectResolution)
	if err != nil {
		return nil
	}
	return declared(file)
}

// declared answers the top level functions without a receiver and the types
// of a file, the names a second declaration in the package collides with.
func declared(file *ast.File) []string {
	var out []string
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil {
				out = append(out, d.Name.Name)
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				if ts, ok := spec.(*ast.TypeSpec); ok {
					out = append(out, ts.Name.Name)
				}
			}
		}
	}
	return out
}

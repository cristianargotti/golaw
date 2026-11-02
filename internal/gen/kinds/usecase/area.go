package usecase

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

// areaNames are the names the use case kind declares once per area, in the
// order a refusal checks them; a package that declares one by hand cannot
// take the generated area file beside it.
func areaNames() []string { return []string{"Deps", "UseCases", "NewUseCases"} }

// areaPackage is what the use case kind reads of the consuming package: its
// clause, its type names and where the area names are declared.
type areaPackage struct {
	Name      string
	Types     map[string]bool
	Generated map[string]string // area name to the first file declaring it
}

// scan reads the non-test Go files directly in dir. It parses without
// types, as every contract reader does (M7.3.1), since a name is all the
// kind needs to know.
func scan(s model.Snapshot, dir, area string) (areaPackage, error) {
	pkg := areaPackage{Name: area, Types: map[string]bool{}, Generated: map[string]string{}}
	files := model.Select(s, func(p string) bool {
		return path.Dir(p) == dir && strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go")
	})
	for i, f := range files {
		content, err := s.Read(f.Path)
		if err != nil {
			return areaPackage{}, fmt.Errorf("usecase %s: read %s: %w", area, f.Path, err)
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), f.Path, content, parser.SkipObjectResolution)
		if err != nil {
			return areaPackage{}, fmt.Errorf("%w: %s does not parse: %w", genkind.ErrCannotJudge, f.Path, err)
		}
		if i == 0 {
			pkg.Name = parsed.Name.Name
		}
		pkg.record(f.Path, parsed)
	}
	return pkg, nil
}

// record notes every type spec and the first declaration of an area name.
func (pkg areaPackage) record(file string, parsed *ast.File) {
	for _, decl := range parsed.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				if ts, ok := spec.(*ast.TypeSpec); ok {
					pkg.Types[ts.Name.Name] = true
					pkg.claim(ts.Name.Name, file)
				}
			}
		case *ast.FuncDecl:
			if d.Recv == nil {
				pkg.claim(d.Name.Name, file)
			}
		}
	}
}

// claim keeps the first file that declares one of the area names.
func (pkg areaPackage) claim(name, file string) {
	for _, area := range areaNames() {
		if name == area && pkg.Generated[name] == "" {
			pkg.Generated[name] = file
		}
	}
}

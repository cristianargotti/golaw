package bootstrap

import (
	"cmp"
	"errors"
	"fmt"
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/model"
)

// PromoteImported keeps inherited records complete without changing go.mod.
func PromoteImported(root string, info ModuleInfo) (ModuleInfo, []assets.Requirement, error) {
	imports, err := moduleImports(root)
	if err != nil {
		return ModuleInfo{}, nil, err
	}
	modules := make([]string, 0, 1+len(info.Direct)+len(info.Indirect))
	modules = append(modules, info.Path)
	for _, requirements := range [][]assets.Requirement{info.Direct, info.Indirect} {
		for _, requirement := range requirements {
			modules = append(modules, requirement.Path)
		}
	}
	used := make(map[string]bool)
	for _, path := range imports {
		if owner := owningModule(path, modules); owner != "" && owner != info.Path {
			used[owner] = true
		}
	}
	var promoted, indirect []assets.Requirement
	info.Direct = slices.Clone(info.Direct)
	for _, requirement := range info.Indirect {
		if used[requirement.Path] {
			promoted = append(promoted, requirement)
			info.Direct = append(info.Direct, requirement)
		} else {
			indirect = append(indirect, requirement)
		}
	}
	info.Indirect = indirect
	for _, requirements := range [][]assets.Requirement{info.Direct, info.Indirect, promoted} {
		slices.SortFunc(requirements, func(a, b assets.Requirement) int { return cmp.Compare(a.Path, b.Path) })
	}
	return info, promoted, nil
}

func moduleImports(root string) ([]string, error) {
	root = filepath.Clean(root)
	var imports []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk %q: %w", path, walkErr)
		}
		if entry.IsDir() {
			return skipImportDirectory(root, path, entry.Name())
		}
		if !entry.Type().IsRegular() || filepath.Ext(path) != ".go" || strings.HasPrefix(entry.Name(), ".") || strings.HasPrefix(entry.Name(), "_") {
			return nil
		}
		paths, err := fileImports(path)
		if err != nil {
			return err
		}
		imports = append(imports, paths...)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("bootstrap: read the module imports: %w", err)
	}
	return imports, nil
}

// skipImportDirectory leaves out what the go command leaves out of the
// module: testdata, vendor, dot and underscore directories, and a nested
// module, which has a graph of its own.
func skipImportDirectory(root, path, name string) error {
	if path == root {
		return nil
	}
	if name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
		return fs.SkipDir
	}
	_, err := os.Lstat(filepath.Join(path, "go.mod"))
	if err == nil {
		return fs.SkipDir
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("inspect nested module %q: %w", path, err)
	}
	return nil
}

// fileImports answers no import for a file over the read bound, one that
// does not parse, or one its build constraint keeps out of go mod tidy:
// the go command refuses or skips such a file too, and a refusal here would
// stop init over a file it never judges.
func fileImports(path string) ([]string, error) {
	file, err := os.Open(path) //nolint:gosec // WalkDir supplies regular files below the module root.
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", path, err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, model.MaxReadBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", path, err)
	}
	if len(data) > model.MaxReadBytes {
		return nil, nil
	}
	parsed, err := parser.ParseFile(token.NewFileSet(), path, data, parser.ImportsOnly|parser.ParseComments)
	if err != nil || !importConstraintsMatch(parsed) {
		return nil, nil
	}
	imports := make([]string, 0, len(parsed.Imports))
	for _, spec := range parsed.Imports {
		name, err := strconv.Unquote(spec.Path.Value)
		if err == nil {
			imports = append(imports, name)
		}
	}
	return imports, nil
}

func importConstraintsMatch(file *ast.File) bool {
	matched := true
	for _, line := range importConstraintLines(file) {
		expression, err := constraint.Parse(line)
		if err != nil {
			return true
		}
		matched = evalImportConstraint(expression, true) && matched
	}
	return matched
}

func importConstraintLines(file *ast.File) []string {
	var modern, legacy []string
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if comment.Pos() >= file.Package {
				continue
			}
			switch {
			case constraint.IsGoBuild(comment.Text):
				modern = append(modern, comment.Text)
			case constraint.IsPlusBuild(comment.Text):
				legacy = append(legacy, comment.Text)
			}
		}
	}
	if len(modern) > 1 {
		return nil
	}
	if len(modern) == 1 {
		return modern
	}
	return legacy
}

// Negation flips the preference because tidy considers every tag except ignore both ways.
func evalImportConstraint(expression constraint.Expr, prefer bool) bool {
	switch expression := expression.(type) {
	case *constraint.TagExpr:
		return expression.Tag != "ignore" && prefer
	case *constraint.NotExpr:
		return !evalImportConstraint(expression.X, !prefer)
	case *constraint.AndExpr:
		return evalImportConstraint(expression.X, prefer) && evalImportConstraint(expression.Y, prefer)
	case *constraint.OrExpr:
		return evalImportConstraint(expression.X, prefer) || evalImportConstraint(expression.Y, prefer)
	}
	return prefer
}

func owningModule(path string, modules []string) string {
	var owner string
	for _, module := range modules {
		if len(module) > len(owner) && (path == module || strings.HasPrefix(path, module+"/")) {
			owner = module
		}
	}
	return owner
}

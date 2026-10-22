package coverage

import (
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// generatedPattern is the header of generated Go code, which has no author
// to hold to the coverage law, in the form scan and the proof read it,
// swaggo's included: an optional Package prefix and no final period.
var generatedPattern = regexp.MustCompile(`(?m)^// (?:Package \S+ )?Code generated .* DO NOT EDIT\.?$`)

func skippedDirs() map[string]bool {
	return map[string]bool{".git": true, "vendor": true, "node_modules": true, "testdata": true, "dist": true}
}

func modulePath(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, "go.mod")) //nolint:gosec // The caller selects the module root.
	if err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if name, ok := moduleDirective(line); ok {
			return name, nil
		}
	}
	return "", ErrModule
}

func moduleDirective(line string) (string, bool) {
	line, _, _ = strings.Cut(line, "//")
	fields := strings.Fields(line)
	if len(fields) != 2 || fields[0] != "module" {
		return "", false
	}
	name := fields[1]
	if strings.HasPrefix(name, `"`) {
		var err error
		name, err = strconv.Unquote(name)
		if err != nil {
			return "", false
		}
	}
	if name == "" || strings.ContainsAny(name, " \t\r\n") {
		return "", false
	}
	return name, true
}

func sourceFiles(root, module string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return sourceDirectory(root, path, d.Name())
		}
		name, ok, err := sourceName(root, module, path)
		if err != nil || !ok {
			return err
		}
		out = append(out, name)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", root, err)
	}
	return out, nil
}

func sourceDirectory(root, path, name string) error {
	if path == root {
		return nil
	}
	if skippedDirs()[name] || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
		return filepath.SkipDir
	}
	// A nested module belongs to its own coverage invocation.
	info, err := os.Stat(filepath.Join(path, "go.mod"))
	if err == nil && !info.IsDir() {
		return filepath.SkipDir
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect module boundary %s: %w", path, err)
	}
	return nil
}

// Each platform measures only the sources its build constraints select.
func sourceName(root, module, path string) (string, bool, error) {
	if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
		return "", false, nil
	}
	built, err := build.Default.MatchFile(filepath.Dir(path), filepath.Base(path))
	if err != nil {
		return "", false, fmt.Errorf("match %s: %w", path, err)
	}
	if !built {
		return "", false, nil
	}
	needs, err := needsCoverage(path)
	if err != nil || !needs {
		return "", false, err
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", false, fmt.Errorf("relative path of %s: %w", path, err)
	}
	return module + "/" + filepath.ToSlash(rel), true, nil
}

func needsCoverage(path string) (bool, error) {
	data, err := os.ReadFile(path) //nolint:gosec // Coverage reads the selected module sources.
	if err != nil {
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	if generatedPattern.Match(data) {
		return false, nil
	}
	file, err := parser.ParseFile(token.NewFileSet(), path, data, parser.SkipObjectResolution)
	if err != nil {
		return false, fmt.Errorf("parse %s: %w", path, err)
	}
	return hasStatements(file), nil
}

// Package-level function literals are instrumented even without function declarations.
func hasStatements(file *ast.File) bool {
	found := false
	ast.Inspect(file, func(n ast.Node) bool {
		switch fn := n.(type) {
		case *ast.FuncDecl:
			found = found || fn.Body != nil && len(fn.Body.List) > 0
		case *ast.FuncLit:
			found = found || len(fn.Body.List) > 0
		}
		return !found
	})
	return found
}

package main

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// generatedPattern is the header of generated Go code, which has no author
// to hold to the coverage law.
var generatedPattern = regexp.MustCompile(`(?m)^// Code generated .* DO NOT EDIT\.$`)

// skippedDirs are the directories that hold no Go package of the module.
func skippedDirs() map[string]bool {
	return map[string]bool{".git": true, "vendor": true, "node_modules": true, "testdata": true, "dist": true}
}

// modulePath reads the module line of go.mod under root.
func modulePath(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, "go.mod")) //nolint:gosec // root is a flag of this tool
	if err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.TrimSpace(rest), nil
		}
	}
	return "", ErrModule
}

// sourceFiles lists, in profile naming (<module>/<relative path>), every
// non-test Go file that declares at least one function with statements.
func sourceFiles(root, module string) ([]string, error) {
	var out []string
	skipped := skippedDirs()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && skipped[d.Name()] {
				return filepath.SkipDir
			}
			return nil
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

// sourceName returns the profile name of a Go source file that needs
// coverage, or false for tests, generated files, files without statements
// and files the build constraints leave out on this operating system (a
// file compiled only on Windows is covered by the Windows runner).
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

// needsCoverage reports whether the file has executable statements and is
// not generated.
func needsCoverage(path string) (bool, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the tool reads the module it checks
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

// hasStatements reports whether a function of the file has a statement, a
// function literal included: the profile measures a file whose code all
// sits in package-level literals, so the floors must see that file too.
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

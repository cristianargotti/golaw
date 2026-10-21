package policyfiles

import (
	"fmt"
	"path"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

const rootModuleFile = "go.mod"

type moduleUnit struct {
	file string
	mod  goModule
}

type dependencyReadError struct {
	file string
	err  error
}

func (e *dependencyReadError) Error() string { return e.err.Error() }
func (e *dependencyReadError) Unwrap() error { return e.err }

type graphModule struct {
	unit     moduleUnit
	requires []moduleRequirement
}

type moduleGraph struct {
	in      model.Input
	cache   map[string]graphModule
	modules []moduleUnit
}

type replacementSet struct {
	file       string
	directives map[[2]string]moduleReplacement
}

// dependencyModules measures the root, optional tools and active local replacements
// because other layouts cannot establish the graph without external resolution.
// Fixtures describe examples rather than application modules and remain unread.
func dependencyModules(in model.Input) ([]moduleUnit, error) {
	files, err := dependencyFiles(in)
	if err != nil {
		return nil, err
	}
	graph := moduleGraph{in: in, cache: make(map[string]graphModule)}
	for _, file := range []string{rootModuleFile, "tools/go.mod"} {
		class := in.Snapshot.Class(file)
		if class == model.ClassFixture || (file != rootModuleFile && class == model.ClassUnknown) {
			continue
		}
		if err := graph.main(file); err != nil {
			return nil, err
		}
	}
	for _, file := range files {
		if _, measured := graph.cache[file]; !measured {
			return nil, outside(file, 1, "a second application module; golaw measures one module at the root, tools/go.mod and the local replacements they name")
		}
	}
	return graph.modules, nil
}

func dependencyFiles(in model.Input) ([]string, error) {
	files := model.Select(in.Snapshot, func(name string) bool {
		if strings.Contains("/"+name, "/vendor/") {
			return moduleVendor(name)
		}
		return path.Base(name) == rootModuleFile || path.Base(name) == "go.work"
	})
	var modules []string
	for _, file := range files {
		if file.Class == model.ClassFixture {
			continue
		}
		if moduleVendor(file.Path) {
			return nil, outside(file.Path, 1, "vendor/ dependency tree")
		}
		if path.Base(file.Path) == "go.work" {
			return nil, outside(file.Path, 1, "go.work dependency workspace")
		}
		modules = append(modules, file.Path)
	}
	return modules, nil
}

func moduleVendor(name string) bool {
	return strings.HasPrefix(name, "vendor/") || strings.HasPrefix(name, "tools/vendor/")
}

func (g *moduleGraph) main(file string) error {
	module, err := g.load(file)
	if err != nil {
		return err
	}
	return g.walk(file, replacementsFor(module.unit), make(map[string]bool))
}

func (g *moduleGraph) load(file string) (graphModule, error) {
	if module, found := g.cache[file]; found {
		return module, nil
	}
	data, found, err := read(g.in, file)
	if err != nil {
		return graphModule{}, &dependencyReadError{file: file, err: err}
	}
	if !found {
		if file == rootModuleFile {
			return graphModule{}, outside(file, 1, "no go.mod at the repository root, which the go capability needs; restore it or remove go from the capabilities")
		}
		return graphModule{}, fmt.Errorf("%s: %w", file, model.ErrNotInSnapshot)
	}
	module, err := readGraphModule(file, data)
	if err != nil {
		return graphModule{}, err
	}
	g.cache[file] = module
	g.modules = append(g.modules, module.unit)
	return module, nil
}

func readGraphModule(file string, data []byte) (graphModule, error) {
	module, err := readModule(file, data)
	if err != nil {
		return graphModule{}, err
	}
	parsed, err := parseModule(file, data)
	if err != nil {
		return graphModule{}, err
	}
	result := graphModule{unit: moduleUnit{file: file, mod: module}}
	for _, required := range parsed.Require {
		result.requires = append(result.requires, moduleRequirement{
			path: required.Mod.Path, version: required.Mod.Version,
		})
	}
	return result, nil
}

func (g *moduleGraph) walk(file string, replacements replacementSet, visited map[string]bool) error {
	if visited[file] || g.in.Snapshot.Class(file) == model.ClassFixture {
		return nil
	}
	visited[file] = true
	module, err := g.load(file)
	if err != nil {
		return err
	}
	for _, required := range module.requires {
		replacement, found := replacements.directives[[2]string{required.path, required.version}]
		if !found {
			replacement, found = replacements.directives[[2]string{required.path, ""}]
		}
		if !found || replacement.newVersion != "" {
			continue
		}
		if err := g.follow(replacement, replacements, visited); err != nil {
			return err
		}
	}
	return nil
}

func (g *moduleGraph) follow(replacement moduleReplacement, replacements replacementSet, visited map[string]bool) error {
	target, err := replacementFile(replacements.file, replacement)
	if err != nil {
		return err
	}
	if g.in.Snapshot.Class(target) == model.ClassUnknown {
		return outside(replacements.file, replacement.line,
			replacementDirective(replacement)+" names a directory without a go.mod in the snapshot")
	}
	return g.walk(target, replacements, visited)
}

func replacementsFor(unit moduleUnit) replacementSet {
	set := replacementSet{file: unit.file, directives: make(map[[2]string]moduleReplacement, len(unit.mod.replaces))}
	for _, replacement := range unit.mod.replaces {
		set.directives[[2]string{replacement.oldPath, replacement.oldVersion}] = replacement
	}
	return set
}

func replacementFile(file string, replacement moduleReplacement) (string, error) {
	target := strings.ReplaceAll(replacement.newPath, "\\", "/")
	directive := replacementDirective(replacement)
	if strings.HasPrefix(target, "/") || (len(target) >= 2 && target[1] == ':') {
		return "", outside(file, replacement.line, directive)
	}
	resolved := path.Clean(path.Join(path.Dir(file), target))
	if resolved == ".." || strings.HasPrefix(resolved, "../") || (len(resolved) >= 2 && resolved[1] == ':') {
		return "", outside(file, replacement.line, directive)
	}
	return path.Join(resolved, rootModuleFile), nil
}

func replacementDirective(replacement moduleReplacement) string {
	original := replacement.oldPath
	if replacement.oldVersion != "" {
		original += " " + replacement.oldVersion
	}
	return fmt.Sprintf("replace %s => %s", original, replacement.newPath)
}

package genmap

import (
	"context"
	"fmt"
	"path"
	"slices"
	"strings"
)

// DefaultContextBudget is the size of an area's context in tokens: about
// what thirty five reads of neighboring files cost an agent, in one turn.
const DefaultContextBudget = 4000

// Words the context reads code by.
const (
	wordExecute = "Execute" // the method every use case of the layout has
	wordNew     = "New"     // the prefix of an exported constructor
	sqlExt      = ".sql"
)

// Decl is one declaration of the context: its name, where it is, and the
// signature lines an agent writes against, never a body.
type Decl struct {
	Name  string   // "cockpit.PlanLedger", "plans", "db/queries/plans.sql"
	At    string   // "internal/application/cockpit/ports.go:61", empty for none
	Lines []string // one signature, field list or route per line
}

// Context is what one area hands an agent before it writes a feature, by
// layer, so it reads the shapes once instead of exploring file by file.
type Context struct {
	Area     string
	Ports    []Decl   // the interfaces of the area with their methods
	Usecases []Decl   // the use cases with constructor, methods, input and output
	Types    []Decl   // the domain types the ports and use cases reference
	Adapters []Decl   // the outbound adapters of the area's ports
	Handlers []Decl   // the inbound handlers that call the area, with routes
	Tables   []Decl   // the columns of the tables the area's queries touch
	Queries  []Decl   // the sqlc queries of the area, per file
	Wiring   []Decl   // where the composition root builds the area's parts
	Patterns []Decl   // per layer, the shortest complete test file to copy
	Commands []string // the golaw new commands that fit the area
}

// areaReader is the state one context is built from.
type areaReader struct {
	in     Input
	files  []goFile
	module string
	area   pkgSrc
	pkgs   map[string]pkgSrc // parsed directories by slash path
	shown  map[string]bool   // "<dir>.<Type>" already printed
	refs   []typeRef         // the types the printed shapes reference
	ports  []portShape
	cases  []string // the use case type names
	words  map[string]bool
	// outDirs and inDirs are the directories of the adapters and the
	// handlers found, which the wiring and the patterns read.
	outDirs, inDirs []string
}

// BuildContext reads one area: its ports, use cases and domain types, the
// adapters and handlers around it, its tables and queries, its wiring and
// the test files to copy, from the snapshot alone with go/parser.
func BuildContext(ctx context.Context, in Input) (Context, error) {
	if err := ctx.Err(); err != nil {
		return Context{}, fmt.Errorf("read the context of %s: %w", in.Area, err)
	}
	r := newAreaReader(in)
	if len(r.area.srcs) == 0 {
		return Context{}, fmt.Errorf("%w: %s", ErrNoArea, in.Area)
	}
	c := Context{Area: in.Area}
	c.Ports = r.portDecls()
	c.Usecases = r.usecaseDecls()
	c.Types = r.typeDecls()
	queries := r.areaQueries()
	c.Adapters = r.adapterDecls(queries)
	c.Handlers = r.handlerDecls()
	c.Tables, c.Queries = r.sqlDecls(queries)
	c.Wiring = r.wiringDecls(c)
	c.Patterns = r.patternDecls(c)
	c.Commands = r.commands(c)
	return c, nil
}

// newAreaReader parses the area's directories and reads the module path.
func newAreaReader(in Input) *areaReader {
	r := &areaReader{in: in, files: goFiles(in.Snapshot), pkgs: map[string]pkgSrc{}, shown: map[string]bool{}, words: map[string]bool{}}
	r.module = modulePath(in)
	dir := layoutDir(in.Policy.Layout.Port, in.Area)
	if in.Policy.Layout.Port == "" {
		dir = layoutDir(in.Policy.Layout.Usecase, in.Area)
	}
	r.area = r.pkg(dir)
	if other := layoutDir(in.Policy.Layout.Usecase, in.Area); other != dir && other != "" {
		for _, s := range r.pkg(other).srcs {
			addSrc(&r.area, s)
		}
	}
	return r
}

// pkg answers the parsed sources of a directory, parsed once.
func (r *areaReader) pkg(dir string) pkgSrc {
	if p, ok := r.pkgs[dir]; ok {
		return p
	}
	p := loadPkg(r.files, dir)
	r.pkgs[dir] = p
	return p
}

// modulePath is the module line of the root go.mod, empty without one.
func modulePath(in Input) string {
	data, err := in.Snapshot.Read("go.mod")
	if err != nil {
		return ""
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.Trim(strings.TrimSpace(rest), `"`)
		}
	}
	return ""
}

// areaImport is the import path of the area's package.
func (r *areaReader) areaImport() string {
	return r.dirImport(r.area.dir)
}

// dirImport is the import path of a directory of the module.
func (r *areaReader) dirImport(dir string) string {
	if r.module == "" {
		return dir
	}
	return r.module + "/" + dir
}

// importDir is the directory of a module import path, false outside it.
func (r *areaReader) importDir(importPath string) (string, bool) {
	if r.module == "" {
		return "", false
	}
	dir, ok := strings.CutPrefix(importPath, r.module+"/")
	return dir, ok
}

// Areas answers every area the layout places a Go source in, sorted, so a
// refusal of an unknown area names the ones there are.
func Areas(in Input) []string {
	var out []string
	for _, f := range goFiles(in.Snapshot) {
		for _, pattern := range []string{in.Policy.Layout.Port, in.Policy.Layout.Usecase} {
			if area, ok := matchLayout(pattern, path.Dir(f.path)); ok && !f.test {
				out = append(out, area)
			}
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

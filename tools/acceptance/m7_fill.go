package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/proof"
)

// m7FillsFile holds the recorded fill of every hole the fixture commands
// open, kept beside the fixture so a kind change that moves a hole fails
// the fixture test instead of an acceptance run.
const m7FillsFile = "fills.yml"

// m7Import is one import a fill needs, named only when the file must use
// another name than the package's own.
type m7Import struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}

// m7Fill is the recorded domain logic of one hole: the whole body of the
// function that holds the marker, written under the receiver and parameter
// names it gives, because a fill replaces what a person would replace.
type m7Fill struct {
	ID      string     `yaml:"id"`
	Recv    string     `yaml:"recv"`
	Params  []string   `yaml:"params"`
	Body    string     `yaml:"body"`
	Imports []m7Import `yaml:"imports"`
}

// m7Fills decodes the recorded fills; an unknown key is an error, so a
// misspelled field never leaves a fill silently empty.
func m7Fills(root string) ([]m7Fill, error) {
	data, err := readFile(filepath.Join(root, filepath.FromSlash(m7FixtureRoot), m7FillsFile+proof.EscapedSuffix))
	if err != nil {
		return nil, err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(proof.Unescape(data)))
	decoder.KnownFields(true)
	var fills []m7Fill
	if err := decoder.Decode(&fills); err != nil {
		return nil, fmt.Errorf("decode the recorded fills: %w", err)
	}
	for i, f := range fills {
		if f.ID == "" || strings.TrimSpace(f.Body) == "" {
			return nil, fmt.Errorf("recorded fill %d has no id or no body: %w", i, ErrAcceptance)
		}
	}
	return fills, nil
}

// m7MarkerOf matches the marker line of one hole id, so an id never
// matches a longer id it prefixes.
func m7MarkerOf(id string) *regexp.Regexp {
	return regexp.MustCompile(`golaw:hole ` + regexp.QuoteMeta(id) + `(\s|$)`)
}

// m7OwnedFile is one owned Go file of a repository as it is now.
type m7OwnedFile struct {
	content []byte
	mode    fs.FileMode
}

// m7OwnedFiles reads the Go files of dir a kind leaves to the person: every
// file but the regenerable zz_generated ones, tests included, because a
// seed hole lives in an owned test file.
func m7OwnedFiles(dir string) (map[string]*m7OwnedFile, error) {
	files := make(map[string]*m7OwnedFile)
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		switch {
		case walkErr != nil:
			return walkErr
		case entry.IsDir() && entry.Name() == ".git":
			return filepath.SkipDir
		case !entry.Type().IsRegular() || !strings.HasSuffix(path, ".go") || strings.HasPrefix(entry.Name(), "zz_generated_"):
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("stat %s: %w", path, err)
		}
		body, err := readFile(path)
		if err != nil {
			return err
		}
		files[path] = &m7OwnedFile{content: body, mode: info.Mode().Perm()}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("read the owned files of %s: %w", dir, err)
	}
	return files, nil
}

// m7ApplyFills writes every fill over the one owned file of dir that holds
// its hole; a hole in no file or in two is an error, because the fixture
// would then prove another slice than the one it records.
func m7ApplyFills(dir string, fills []m7Fill) error {
	files, err := m7OwnedFiles(dir)
	if err != nil {
		return err
	}
	for _, fill := range fills {
		marker := m7MarkerOf(fill.ID)
		var holders []string
		for path, file := range files {
			if marker.Match(file.content) {
				holders = append(holders, path)
			}
		}
		if len(holders) != 1 {
			return fmt.Errorf("hole %s is in %d files: %w", fill.ID, len(holders), ErrAcceptance)
		}
		file := files[holders[0]]
		filled, err := m7FillFunc(file.content, fill)
		if err != nil {
			return err
		}
		if err := writeFile(holders[0], filled, file.mode); err != nil {
			return err
		}
		file.content = filled
	}
	return nil
}

// m7Edit replaces the byte range [from, to) of a file with text.
type m7Edit struct {
	from, to int
	text     string
}

// m7Replace applies edits that never overlap, the last first, so every
// offset still points where it was read.
func m7Replace(content []byte, edits []m7Edit) []byte {
	slices.SortFunc(edits, func(a, b m7Edit) int { return b.from - a.from })
	out := bytes.Clone(content)
	for _, e := range edits {
		out = slices.Concat(out[:e.from], []byte(e.text), out[e.to:])
	}
	return out
}

// m7FillFunc replaces the body of the function that holds the hole of fill
// and renames its receiver and parameters to the fill's names, so the body
// reads as it was recorded whatever names the kind chose.
func m7FillFunc(content []byte, fill m7Fill) ([]byte, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", content, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse the file of hole %s: %w", fill.ID, err)
	}
	fn := m7HoleFunc(file, fill.ID)
	if fn == nil {
		return nil, fmt.Errorf("no function holds hole %s: %w", fill.ID, ErrAcceptance)
	}
	edits, err := m7Renames(fset, fn, fill)
	if err != nil {
		return nil, err
	}
	at := func(pos token.Pos) int { return fset.Position(pos).Offset }
	edits = append(edits, m7Edit{from: at(fn.Body.Lbrace) + 1, to: at(fn.Body.Rbrace), text: "\n" + fill.Body + "\n"})
	out, err := m7AddImports(m7Replace(content, edits), fill.Imports)
	if err == nil {
		out, err = m7PruneImports(out)
	}
	if err != nil {
		return nil, fmt.Errorf("fill hole %s: %w", fill.ID, err)
	}
	return out, nil
}

// m7HoleFunc answers the function whose body holds the marker of id.
func m7HoleFunc(file *ast.File, id string) *ast.FuncDecl {
	marker := m7MarkerOf(id)
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		for _, group := range file.Comments {
			for _, c := range group.List {
				if c.Pos() > fn.Body.Lbrace && c.Pos() < fn.Body.Rbrace && marker.MatchString(c.Text) {
					return fn
				}
			}
		}
	}
	return nil
}

// m7Renames are the edits that give the receiver and every parameter of fn
// the names of fill, in the order of the signature.
func m7Renames(fset *token.FileSet, fn *ast.FuncDecl, fill m7Fill) ([]m7Edit, error) {
	var names []*ast.Ident
	if fn.Recv != nil && len(fn.Recv.List) == 1 && len(fn.Recv.List[0].Names) == 1 {
		names = append(names, fn.Recv.List[0].Names[0])
	}
	if hasRecv := len(names) == 1; hasRecv != (fill.Recv != "") {
		return nil, fmt.Errorf("hole %s: the fill names the receiver %q, the function has a named receiver %v: %w", fill.ID, fill.Recv, hasRecv, ErrAcceptance)
	}
	var params []*ast.Ident
	for _, field := range fn.Type.Params.List {
		params = append(params, field.Names...)
	}
	if len(params) != len(fill.Params) {
		return nil, fmt.Errorf("the signature of hole %s names %d parameters, the fill %d: %w", fill.ID, len(params), len(fill.Params), ErrAcceptance)
	}
	want := append([]string{fill.Recv}, fill.Params...)
	if len(names) == 0 {
		want = want[1:]
	}
	edits := make([]m7Edit, 0, len(want))
	for i, ident := range append(names, params...) {
		from := fset.Position(ident.Pos()).Offset
		edits = append(edits, m7Edit{from: from, to: from + len(ident.Name), text: want[i]})
	}
	return edits, nil
}

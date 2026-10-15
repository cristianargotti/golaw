package goast

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"

	"github.com/cristianargotti/golaw/internal/model"
)

const (
	codeGoleak    = "goleak"
	goleakModule  = "go.uber.org/goleak"
	goroutineFunc = "Go"
)

// Goleak is goast.goleak: a package that starts goroutines, through a go
// statement or a Go method such as sync.WaitGroup.Go, carries a test that
// imports goleak. The finding is reported once per package, at the first
// start the run meets.
func Goleak() model.Check {
	return check{spec: spec("goleak"), judge: judgeGoleak}
}

func judgeGoleak(t *target, s *state) ([]model.Finding, error) {
	file, _, err := t.parsed()
	if err != nil {
		return nil, err
	}
	start := goroutineStart(file)
	if !start.IsValid() || !s.first("goleak:"+t.dir()) {
		return nil, nil
	}
	verified, err := s.verifiesGoleak(t.dir())
	if err != nil || verified {
		return nil, err
	}
	message := "package " + t.dir() + " starts goroutines and no test of the package verifies with goleak"
	return []model.Finding{t.finding(codeGoleak, start, message)}, nil
}

// goroutineStart is the position of the first go statement or Go method
// call of a file, token.NoPos when it starts none.
func goroutineStart(file *ast.File) token.Pos {
	found := token.NoPos
	ast.Inspect(file, func(n ast.Node) bool {
		if found.IsValid() {
			return false
		}
		switch e := n.(type) {
		case *ast.GoStmt:
			found = e.Pos()
		case *ast.CallExpr:
			if lastSegment(callee(e)) == goroutineFunc && callee(e) != goroutineFunc {
				found = e.Pos()
			}
		}
		return true
	})
	return found
}

// verifiesGoleak reports whether a test file of the directory imports
// goleak.
func (s *state) verifiesGoleak(dir string) (bool, error) {
	for _, f := range s.siblings(dir) {
		if f.Class != model.ClassTest {
			continue
		}
		src, err := s.in.Snapshot.Read(f.Path)
		if err != nil {
			return false, fmt.Errorf("read %s: %w", f.Path, err)
		}
		file, err := parser.ParseFile(token.NewFileSet(), f.Path, src, parser.ImportsOnly)
		if err != nil {
			return false, parseError(f.Path, err)
		}
		for _, imp := range file.Imports {
			if p, err := strconv.Unquote(imp.Path.Value); err == nil && p == goleakModule {
				return true, nil
			}
		}
	}
	return false, nil
}

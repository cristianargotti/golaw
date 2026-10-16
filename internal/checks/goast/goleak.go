package goast

import (
	"errors"
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
	verified, unread, err := s.verifiesGoleak(t.dir())
	if err != nil || verified {
		return nil, err
	}
	if len(unread) > 0 {
		// The import may sit in a test over the read bound.
		s.leave(unread...)
		return nil, nil
	}
	message := "package " + t.dir() + " starts goroutines and no test of the package verifies with goleak"
	return []model.Finding{t.finding(codeGoleak, start, message)}, nil
}

// goroutineStart stops at the first start: one is enough to need goleak,
// and the finding sits there.
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

// verifiesGoleak parses the imports only, because importing goleak in a
// test of the package is all the rule asks; a test over the read bound is
// answered aside, as one that may import it.
func (s *state) verifiesGoleak(dir string) (bool, []string, error) {
	var unread []string
	for _, f := range s.siblings(dir) {
		if f.Class != model.ClassTest {
			continue
		}
		src, err := s.in.Snapshot.Read(f.Path)
		if errors.Is(err, model.ErrTooLarge) {
			unread = append(unread, f.Path)
			continue
		}
		if err != nil {
			return false, nil, fmt.Errorf("read %s: %w", f.Path, err)
		}
		file, err := parser.ParseFile(token.NewFileSet(), f.Path, src, parser.ImportsOnly)
		if err != nil {
			return false, nil, parseError(f.Path, err)
		}
		for _, imp := range file.Imports {
			if p, err := strconv.Unquote(imp.Path.Value); err == nil && p == goleakModule {
				return true, nil, nil
			}
		}
	}
	return false, unread, nil
}

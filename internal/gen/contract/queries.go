package contract

import (
	"cmp"
	"fmt"
	"go/ast"
	"path"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
)

// sqlcQueries is the receiver every sqlc query method has.
const sqlcQueries = "Queries"

// Queries answers the query methods of the sqlc output in dir, sorted by
// name: every method on *Queries in a .sql.go file of that directory, so the
// constructor and WithTx of db.go are never taken for a query. A directory
// without output answers nil, and the adapter kind decides what that means.
func (r Reader) Queries(dir string) ([]genkind.Query, error) {
	var out []genkind.Query
	for _, file := range r.Snapshot.Files() {
		if path.Dir(file.Path) != dir || !strings.HasSuffix(file.Path, ".sql.go") {
			continue
		}
		content, err := r.Snapshot.Read(file.Path)
		if err != nil {
			return nil, fmt.Errorf("read sqlc output %s: %w", file.Path, err)
		}
		queries, err := queriesOf(file.Path, content)
		if err != nil {
			return nil, err
		}
		out = append(out, queries...)
	}
	slices.SortStableFunc(out, func(a, b genkind.Query) int { return cmp.Compare(a.Name, b.Name) })
	return out, nil
}

// queriesOf answers the query methods one sqlc file declares, the context
// parameter left out because the adapter passes its own.
func queriesOf(rel string, content []byte) ([]genkind.Query, error) {
	file, fset, err := parse(rel, content)
	if err != nil {
		return nil, err
	}
	var out []genkind.Query
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || !onQueries(fn) {
			continue
		}
		in := params(fset, content, fn.Type.Params)
		if len(in) > 0 && in[0].Type == "context.Context" {
			// A query that takes the context alone has no parameter left,
			// spelled nil like a parameter list the contract leaves empty.
			in = in[1:]
			if len(in) == 0 {
				in = nil
			}
		}
		out = append(out, genkind.Query{
			Name: fn.Name.Name, Params: in, Results: params(fset, content, fn.Type.Results), File: path.Base(rel),
		})
	}
	return out, nil
}

// onQueries reports a method whose receiver is *Queries.
func onQueries(fn *ast.FuncDecl) bool {
	if fn.Recv == nil || len(fn.Recv.List) != 1 {
		return false
	}
	star, ok := fn.Recv.List[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	ident, ok := star.X.(*ast.Ident)
	return ok && ident.Name == sqlcQueries
}

package adaptersql

import (
	"fmt"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/kinds/adapterkit"
)

// stubs renders the method stubs and the mappers of the owned main file and
// answers the hole ids they open, methods first.
func (r renderer) stubs(methods []method, mappers []mapper) ([][]byte, []string, error) {
	var decls [][]byte
	var ids []string
	for i := range methods {
		decl, err := adapterkit.Decl(r.tmpl, "method.go.tmpl", stub{D: r.p.data, M: methods[i]})
		if err != nil {
			return nil, nil, fmt.Errorf(wrapped, r.p.target.Name, err)
		}
		decls, ids = append(decls, decl), append(ids, methods[i].Hole)
	}
	for _, mp := range mappers {
		decl, err := adapterkit.Decl(r.tmpl, "mapper.go.tmpl", mapperStub{D: r.p.data, P: mp})
		if err != nil {
			return nil, nil, fmt.Errorf(wrapped, r.p.target.Name, err)
		}
		decls, ids = append(decls, decl), append(ids, mp.Hole)
	}
	return decls, ids, nil
}

// reds renders the red test of each method, run over the scripted driver.
func (r renderer) reds(methods []method) ([][]byte, error) {
	var decls [][]byte
	for i := range methods {
		decl, err := adapterkit.Decl(r.tmpl, "red_test.go.tmpl", stub{D: r.p.data, M: methods[i]})
		if err != nil {
			return nil, fmt.Errorf(wrapped, r.p.target.Name, err)
		}
		decls = append(decls, decl)
	}
	return decls, nil
}

// mapperReds renders the red test of each mapper, called with a zero row.
func (r renderer) mapperReds(mappers []mapper) ([][]byte, error) {
	var decls [][]byte
	for _, mp := range mappers {
		decl, err := adapterkit.Decl(r.tmpl, "mapper_red_test.go.tmpl", mapperStub{D: r.p.data, P: mp})
		if err != nil {
			return nil, fmt.Errorf(wrapped, r.p.target.Name, err)
		}
		decls = append(decls, decl)
	}
	return decls, nil
}

// mainImports are what the stubs and the mappers name, and nothing more.
func (r renderer) mainImports(methods []method, mappers []mapper) []genkind.Import {
	var exprs []string
	for i := range methods {
		for _, p := range append(append([]genkind.Param{}, methods[i].Params...), methods[i].Results...) {
			exprs = append(exprs, p.Type)
		}
	}
	for _, mp := range mappers {
		exprs = append(exprs, mp.Domain)
	}
	out := append(adapterkit.Uses(r.p.imports, exprs), r.support())
	if len(mappers) > 0 {
		out = append(out, r.sqlc())
	}
	return out
}

// testImports are what the red tests of methods name.
func (r renderer) testImports(methods []method) []genkind.Import {
	var exprs []string
	for i := range methods {
		for _, p := range methods[i].Inputs {
			exprs = append(exprs, p.Type)
		}
	}
	return append(adapterkit.Uses(r.p.imports, exprs),
		genkind.Import{Path: sqlPath}, genkind.Import{Path: errorsPath}, genkind.Import{Path: testingPath},
		r.support(), genkind.Import{Path: r.support().Path + "/sqltest"}, r.p.target.Package)
}

// internalImports are what the red tests of the mappers name.
func (r renderer) internalImports() []genkind.Import {
	return []genkind.Import{{Path: errorsPath}, {Path: testingPath}, r.support(), r.sqlc()}
}

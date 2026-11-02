package adaptersql

import (
	"errors"
	"fmt"
	"go/token"
	"path"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/holes"
	"github.com/cristianargotti/golaw/internal/gen/kinds/adapterkit"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/gen/render"
)

// queriesDir is the queries directory of the kit's sqlc.yaml, where the
// statement an sqlcgen file was generated from lives.
const queriesDir = "db/queries/"

// data is what every template of the kind reads: the names settled once, so
// a template only joins strings.
type data struct {
	Package, Type, Lower, Recv, Port, PortType, PortPkg, SQLC, Suite, Seed, SeedConstraint string

	Methods []method
	Mappers []mapper
}

// method is one port method with the constraint its hole carries.
type method struct {
	adapterkit.Method

	Constraint string
}

// mapper is one shared row mapping function: every method reading the same
// domain type calls it, so a second instance never copies the conversion.
type mapper struct {
	Name, Model, Domain, Hole, Constraint, Test, Message string
}

// stub and mapperStub are the data of one declaration template.
type stub struct {
	D data
	M method
}

type mapperStub struct {
	D data
	P mapper
}

// sqlPlan is one adapter resolved against its port, the sqlc output and the
// code its package already declares.
type sqlPlan struct {
	target     adapterkit.Target
	data       data
	imports    []genkind.Import // what the port's types need, from adapterkit.Methods
	pkg        adapterkit.Package
	contracts  []manifest.Contract
	newMappers []mapper // the mappers the package does not declare yet
}

// plan reads everything the renderings need; it is pure over the input.
func plan(in genkind.Input, t adapterkit.Target) (sqlPlan, error) {
	shaped, imports, err := adapterkit.Methods(t)
	if err != nil {
		return sqlPlan{}, fmt.Errorf(wrapped, t.Name, err)
	}
	queries, err := sqlcQueries(in)
	if err != nil {
		return sqlPlan{}, fmt.Errorf(wrapped, t.Name, err)
	}
	pkg, err := adapterkit.Scan(in.Snapshot, t.Dir, t.Name.Type)
	if err != nil {
		return sqlPlan{}, fmt.Errorf(wrapped, t.Name, err)
	}
	sentinels, err := adapterkit.Sentinels(in.Snapshot, t.Port, "NotFound")
	if err != nil {
		return sqlPlan{}, fmt.Errorf(wrapped, t.Name, err)
	}
	p := sqlPlan{target: t, imports: imports, pkg: pkg, data: names(t, path.Base(in.Policy.ThirdParty.SQLCOut))}
	p.match(shaped, queries, adapterkit.Sentence(sentinels, t.Port.Dir))
	return p, nil
}

// sqlcQueries reads the sqlc output; no output means sqlc has not run yet,
// which golaw gen fixes, so the refusal names that command.
func sqlcQueries(in genkind.Input) ([]genkind.Query, error) {
	out := in.Policy.ThirdParty.SQLCOut
	queries, err := in.Contracts.Queries(out)
	if errors.Is(err, genkind.ErrCannotJudge) {
		return nil, fmt.Errorf("sqlc output %s: %w", out, err)
	}
	if err != nil || len(queries) == 0 {
		return nil, fmt.Errorf("%w: no sqlc output in %s: run golaw gen, which runs sqlc, then golaw new again",
			genkind.ErrRequest, out)
	}
	return queries, nil
}

// names settles the identifiers of the templates.
func names(t adapterkit.Target, sqlc string) data {
	d := data{
		Package: t.Name.Area, Type: t.Name.Type, Lower: t.Lower(), Recv: t.Recv,
		Port: t.Port.Package + "." + t.Port.Name, PortType: t.Port.Name, PortPkg: t.Port.Package,
		SQLC: sqlc, Suite: path.Base(t.Suite.Path), Seed: holes.ID("seed", t.Lower()),
	}
	d.SeedConstraint = seedConstraint(d)
	return d
}

// match pairs each method with the query of its name and its mapper, and
// collects the contracts the adapter is generated from.
func (p *sqlPlan) match(shaped []adapterkit.Method, queries []genkind.Query, sentinel string) {
	files := []string{}
	for k := range shaped {
		m := shaped[k]
		i := slices.IndexFunc(queries, func(q genkind.Query) bool { return q.Name == m.Name })
		var q *genkind.Query
		mapperName := ""
		if i >= 0 {
			q = &queries[i]
			files = append(files, queriesDir+strings.TrimSuffix(q.File, ".go"))
			if mp, ok := p.mapperOf(&m, *q, sentinel); ok {
				mapperName = mp.Name
				p.addMapper(mp)
			}
		}
		p.data.Methods = append(p.data.Methods, method{Method: m, Constraint: methodConstraint(p.target.Recv, m, q, mapperName, sentinel)})
	}
	slices.Sort(files)
	p.contracts = []manifest.Contract{p.target.Contract}
	for _, f := range slices.Compact(files) {
		p.contracts = append(p.contracts, manifest.Contract{Path: f})
	}
}

// mapperOf answers the mapper of a method with one result over a query that
// answers one sqlc model or a list of them; any other pair has no row to map.
func (p *sqlPlan) mapperOf(m *adapterkit.Method, q genkind.Query, sentinel string) (mapper, bool) {
	if len(m.Zeros) != 1 || len(q.Results) != 2 || q.Results[1].Type != "error" {
		return mapper{}, false
	}
	model := strings.TrimPrefix(q.Results[0].Type, "[]")
	if !token.IsIdentifier(model) || !token.IsExported(model) {
		return mapper{}, false
	}
	domain := strings.TrimPrefix(m.Zeros[0].Type, "[]")
	base, ok := namedType(domain)
	if !ok {
		return mapper{}, false
	}
	name := render.LowerFirst(base) + "Of"
	mp := mapper{
		Name: name, Model: p.data.SQLC + "." + model, Domain: domain,
		Hole: p.target.Hole(name), Test: "Test" + upperFirst(name),
	}
	mp.Constraint = mapperConstraint(mp, sentinel)
	mp.Message = strconv.Quote("hole " + mp.Hole + " is open: " + mp.Constraint)
	return mp, true
}

// addMapper keeps one mapper per name, fresh only when the package lacks it.
func (p *sqlPlan) addMapper(mp mapper) {
	if slices.ContainsFunc(p.data.Mappers, func(o mapper) bool { return o.Name == mp.Name }) {
		return
	}
	p.data.Mappers = append(p.data.Mappers, mp)
	if _, declared := p.pkg.Funcs[mp.Name]; !declared {
		p.newMappers = append(p.newMappers, mp)
	}
}

// namedType answers the type name of a named type or a pointer to one, the
// only domain shapes one row converts into.
func namedType(domain string) (string, bool) {
	parts := strings.Split(strings.TrimPrefix(domain, "*"), ".")
	for _, part := range parts {
		if !token.IsIdentifier(part) {
			return "", false
		}
	}
	base := parts[len(parts)-1]
	return base, len(parts) <= 2 && token.IsExported(base)
}

func upperFirst(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}

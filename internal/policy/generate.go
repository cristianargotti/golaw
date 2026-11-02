package policy

import (
	"path"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// Generate is the generate block of golaw.yml (M7.6.2): where the
// generators write, outside the floors and the digest (A43). Every key is
// optional; an absent key takes model.DefaultGenerate.
type Generate struct {
	Layout     *GenerateLayout `yaml:"layout" json:"layout"`
	Support    *string         `yaml:"support" json:"support"`
	ThirdParty *ThirdParty     `yaml:"third_party" json:"third_party"`
}

// GenerateLayout is the layout block of generate, one path pattern per kind.
type GenerateLayout struct {
	Port        *string `yaml:"port" json:"port"`
	Usecase     *string `yaml:"usecase" json:"usecase"`
	AdapterSQL  *string `yaml:"adapter_sql" json:"adapter_sql"`
	AdapterHTTP *string `yaml:"adapter_http" json:"adapter_http"`
	Handler     *string `yaml:"handler" json:"handler"`
	Wiring      *string `yaml:"wiring" json:"wiring"`
}

// ThirdParty names the inputs and outputs of the pinned generators.
type ThirdParty struct {
	SQLCOut    *string `yaml:"sqlc_out" json:"sqlc_out"`
	OpenAPI    *string `yaml:"openapi" json:"openapi"`
	OpenAPIOut *string `yaml:"openapi_out" json:"openapi_out"`
}

// The placeholders a layout pattern may hold, one per key.
const (
	placeArea    = "{area}"
	placePackage = "{package}"
	placeMain    = "{main}"
)

// generateRow is one key of the block: what the file sets, what the run
// resolves, the one placeholder the key takes and the last element its
// directory must carry, so validation and explanation read one table.
type generateRow struct {
	key         string
	set         *string
	resolved    string
	placeholder string
	last        string
}

// Resolve answers the block over the defaults: each set key replaces its
// default, and an absent sqlc_out follows adapter_sql so a repository that
// moved its adapters never gets the sqlc output in a second tree.
func (g *Generate) Resolve() model.GeneratePolicy {
	out := model.DefaultGenerate()
	if g == nil {
		return out
	}
	out.Declared = true
	l, tp := g.parts()
	for _, f := range []struct{ dst, src *string }{
		{&out.Layout.Port, l.Port},
		{&out.Layout.Usecase, l.Usecase},
		{&out.Layout.AdapterSQL, l.AdapterSQL},
		{&out.Layout.AdapterHTTP, l.AdapterHTTP},
		{&out.Layout.Handler, l.Handler},
		{&out.Layout.Wiring, l.Wiring},
		{&out.Support, g.Support},
		{&out.ThirdParty.SQLCOut, tp.SQLCOut},
		{&out.ThirdParty.OpenAPI, tp.OpenAPI},
		{&out.ThirdParty.OpenAPIOut, tp.OpenAPIOut},
	} {
		if f.src != nil {
			*f.dst = *f.src
		}
	}
	if tp.SQLCOut == nil {
		out.ThirdParty.SQLCOut = strings.Replace(out.Layout.AdapterSQL, placePackage, "postgres", 1) + "/sqlcgen"
	}
	return out
}

// parts answers the two nested blocks, empty where the file leaves one out.
func (g *Generate) parts() (GenerateLayout, ThirdParty) {
	var l GenerateLayout
	var tp ThirdParty
	if g.Layout != nil {
		l = *g.Layout
	}
	if g.ThirdParty != nil {
		tp = *g.ThirdParty
	}
	return l, tp
}

// rows answers every key of a non nil block in the order explain prints.
func (g *Generate) rows(r model.GeneratePolicy) []generateRow {
	l, tp := g.parts()
	return []generateRow{
		{"layout.port", l.Port, r.Layout.Port, placeArea, ""},
		{"layout.usecase", l.Usecase, r.Layout.Usecase, placeArea, ""},
		{"layout.adapter_sql", l.AdapterSQL, r.Layout.AdapterSQL, placePackage, ""},
		{"layout.adapter_http", l.AdapterHTTP, r.Layout.AdapterHTTP, placePackage, ""},
		{"layout.handler", l.Handler, r.Layout.Handler, placePackage, ""},
		{"layout.wiring", l.Wiring, r.Layout.Wiring, placeMain, ""},
		{"support", g.Support, r.Support, "", ""},
		{"third_party.sqlc_out", tp.SQLCOut, r.ThirdParty.SQLCOut, "", "sqlcgen"},
		{"third_party.openapi", tp.OpenAPI, r.ThirdParty.OpenAPI, "", ""},
		{"third_party.openapi_out", tp.OpenAPIOut, r.ThirdParty.OpenAPIOut, "", "gen"},
	}
}

// validate refuses a block the generators could not render: a set value
// that is not a clean relative slash path, that holds a placeholder its
// key does not take, or whose generated output directory carries another
// name than the one the source classes recognize (A49).
func (g *Generate) validate() error {
	if g == nil {
		return nil
	}
	for _, row := range g.rows(g.Resolve()) {
		if row.set == nil {
			continue
		}
		if err := row.check(*row.set); err != nil {
			return err
		}
	}
	return nil
}

// check judges one set value; the allowed placeholder stands in as a plain
// element so the path rules read the shape the generator will write.
func (r generateRow) check(value string) error {
	shaped := value
	if r.placeholder != "" {
		shaped = strings.ReplaceAll(value, r.placeholder, "p")
	}
	if strings.ContainsAny(shaped, "{}") {
		if r.placeholder == "" {
			return invalid("generate.%s %q takes no placeholder", r.key, value)
		}
		return invalid("generate.%s %q holds a placeholder other than %s", r.key, value, r.placeholder)
	}
	cleaned := path.Clean(shaped)
	if strings.HasPrefix(shaped, "/") || strings.Contains(shaped, `\`) || cleaned != shaped ||
		cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return invalid("generate.%s %q is not a clean relative slash path", r.key, value)
	}
	if r.last != "" && path.Base(cleaned) != r.last {
		return invalid("generate.%s %q must end in a directory named %s", r.key, value, r.last)
	}
	return nil
}

// generate explains every key of a declared block: the file origin for a
// key it sets and the core default for the rest; a policy without the
// block explains none, so config explain of an M6 repository is unchanged.
func (e *explainer) generate(f File, p *model.EffectivePolicy) {
	if f.Generate == nil {
		return
	}
	for _, row := range f.Generate.rows(p.Generate) {
		e.set("generate."+row.key, row.resolved, row.set != nil)
	}
}

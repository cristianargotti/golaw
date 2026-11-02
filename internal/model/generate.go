package model

// GenerateLayout is the layout block of golaw.yml generate (M7.6.2): each
// value a slash path pattern with {area}, {package} or {main}, so the
// generators write beside the code a repository already holds instead of
// opening a second tree.
type GenerateLayout struct {
	Port        string // the directory of a port, with {area}
	Usecase     string // the directory of a use case, with {area}
	AdapterSQL  string // the directory of a database adapter, with {package}
	AdapterHTTP string // the directory of an HTTP client adapter, with {package}
	Handler     string // the directory of an HTTP handler, with {package}
	Wiring      string // the directory of the composition root, with {main}
}

// GeneratePaths are the inputs and outputs of the pinned third party
// generators; their last element is fixed (A49) so the source class of a
// generated tree never depends on a repository's naming.
type GeneratePaths struct {
	SQLCOut    string // the sqlc output directory, named sqlcgen
	OpenAPI    string // the OpenAPI contract the HTTP generator reads
	OpenAPIOut string // the oapi-codegen output directory, named gen
}

// GeneratePolicy is the generate block as the run reads it.
type GeneratePolicy struct {
	Layout     GenerateLayout
	Support    string // the support package every generated part calls
	ThirdParty GeneratePaths
	Declared   bool // golaw.yml holds a generate block; false means the defaults answer
}

// DefaultGenerate is the block of a policy that names none: the defaults of
// M7.6.2.
func DefaultGenerate() GeneratePolicy {
	return GeneratePolicy{
		Layout: GenerateLayout{
			Port:        "internal/application/{area}",
			Usecase:     "internal/application/{area}",
			AdapterSQL:  "internal/adapters/out/{package}",
			AdapterHTTP: "internal/adapters/out/{package}",
			Handler:     "internal/adapters/in/{package}",
			Wiring:      "cmd/{main}",
		},
		Support: "internal/gensupport",
		ThirdParty: GeneratePaths{
			SQLCOut:    "internal/adapters/out/postgres/sqlcgen",
			OpenAPI:    "api/openapi.yaml",
			OpenAPIOut: "api/gen",
		},
	}
}

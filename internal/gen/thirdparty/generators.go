package thirdparty

import (
	"bytes"
	"path"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// The names of the pinned generators, as runner.Generators lists them and
// as go tool knows them in the tools module.
const (
	nameSQLC     = "sqlc"
	nameOAPI     = "oapi-codegen"
	nameGoverter = "goverter"
)

// The managed configurations golaw init writes (the destinations of
// templates/manifest.yml); a generator runs only over one the checkout
// holds, so a repository that never adopted it is left alone.
const (
	sqlcConfig      = "sqlc.yaml"
	oapiServerFile  = "api/codegen/server.yaml"
	oapiSpecFile    = "api/codegen/spec.yaml"
	oapiTypesFile   = "api/codegen/types.yaml"
	goverterMarker  = "// goverter:converter"
	converterFile   = "convert.go"
	sqlcQueriesGlob = "db/queries/*.sql"
	sqlcSchemaGlob  = "db/migrations/*.sql"
)

// oapiConfigs is the run order of the codegen configurations.
func oapiConfigs() []string { return []string{oapiServerFile, oapiSpecFile, oapiTypesFile} }

// Generator is one run of a pinned generator over one configuration.
type Generator struct {
	Name   string // the tool name go tool resolves in the tools module
	Config string // the configuration it reads; for goverter the converter file
	// Args follow the binary; they name the configuration so the run never
	// depends on a generator's discovery of a default file.
	Args []string
	// Inputs are slash globs of the files that feed the run; a change to one
	// of them is what makes the hook regenerate.
	Inputs []string
	// Outputs are the directories the run owns; empty when the generator
	// writes where its own source says, as goverter does.
	Outputs []string
	// Hook is false for goverter because it loads the repository's
	// packages, which the post hook never does inside its budget (A44).
	Hook bool
}

// Generators answers, in run order, the pinned generators the policy's
// capabilities enable over the configurations the snapshot holds.
func Generators(p *model.EffectivePolicy, s model.Snapshot) []Generator {
	if p == nil {
		return nil
	}
	var out []Generator
	paths := p.Generate.ThirdParty
	if p.Has("db") && present(s, sqlcConfig) {
		out = append(out, Generator{
			Name: nameSQLC, Config: sqlcConfig, Args: []string{"generate", "-f", sqlcConfig},
			Inputs: []string{sqlcQueriesGlob, sqlcSchemaGlob}, Outputs: []string{paths.SQLCOut}, Hook: true,
		})
	}
	if p.Has("api") {
		for _, config := range oapiConfigs() {
			if !present(s, config) {
				continue
			}
			out = append(out, Generator{
				Name: nameOAPI, Config: config, Args: []string{"-config", config, paths.OpenAPI},
				Inputs: []string{paths.OpenAPI}, Outputs: []string{paths.OpenAPIOut}, Hook: true,
			})
		}
	}
	if p.Has("hexagonal") {
		out = append(out, converters(s)...)
	}
	return out
}

// present reports a path the snapshot holds in any class.
func present(s model.Snapshot, rel string) bool {
	return s.Class(rel) != model.ClassUnknown
}

// converters answers one goverter run per directory whose convert.go
// carries the converter marker. Only files of that name are read, so the
// hook, which asks on every regenerating event, reads no other file.
func converters(s model.Snapshot) []Generator {
	var out []Generator
	files := model.Select(s, func(p string) bool { return path.Base(p) == converterFile })
	for _, file := range files {
		if file.Class != model.ClassTracked && file.Class != model.ClassUntracked {
			continue
		}
		content, err := s.Read(file.Path)
		if err != nil || !marked(content) {
			continue
		}
		out = append(out, Generator{
			Name: nameGoverter, Config: file.Path, Args: []string{"gen", "./" + path.Dir(file.Path)},
			Inputs: []string{file.Path},
		})
	}
	return out
}

// marked reports a line equal to the goverter marker once trimmed, so a
// mention of the marker inside other text never enables a run.
func marked(content []byte) bool {
	for line := range bytes.Lines(content) {
		if strings.TrimSpace(string(line)) == goverterMarker {
			return true
		}
	}
	return false
}

// Applicable reports a generator the checkout uses: every input glob
// matches a file, or an output directory holds one. A managed
// configuration whose contract was never written, with nothing generated,
// is a repository that has not opted into the generator yet.
func Applicable(s model.Snapshot, g Generator) bool {
	inputs := len(g.Inputs) > 0
	for _, input := range g.Inputs {
		inputs = inputs && model.Any(s, func(p string) bool { ok, err := path.Match(input, p); return err == nil && ok })
	}
	if inputs {
		return true
	}
	return slices.ContainsFunc(g.Outputs, func(out string) bool {
		return model.Any(s, func(p string) bool { return strings.HasPrefix(p, out+"/") })
	})
}

// Triggered keeps, in their order, the generators one of whose inputs
// matches a changed path. path.Match never crosses a slash, so a nested
// file is not an input of a flat glob; a malformed glob matches nothing.
func Triggered(generators []Generator, changed []string) []Generator {
	var out []Generator
	for _, g := range generators {
		if slices.ContainsFunc(changed, func(p string) bool { return feeds(g, p) }) {
			out = append(out, g)
		}
	}
	return out
}

// feeds reports whether a changed path is one of the generator's inputs.
func feeds(g Generator, changed string) bool {
	for _, input := range g.Inputs {
		if ok, err := path.Match(input, changed); err == nil && ok {
			return true
		}
	}
	return false
}

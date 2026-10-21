package rung

import (
	"path"
	"slices"
	"strings"
)

// The generator configurations whose sources the generate-check step reads.
const (
	mockeryConfig = ".mockery.yaml"
	sqlcConfig    = "sqlc.yaml"
	codegenDir    = "api/codegen/"
)

// pipelineStep is a step of section 7 this version leaves to the make
// target of its name (D13): its tool is not in the pinned toolchain setup
// installs, so the rung names the step while its capability holds and the
// commit stages its sources, and runs nothing for it.
type pipelineStep struct {
	name       string
	capability string
	text       string
	applies    func(tracked func(func(string) bool) bool, staged []string) bool
}

// pipelineSteps are the three steps in the order of section 7.
func pipelineSteps() []pipelineStep {
	return []pipelineStep{
		{"migrate-lint", "db", "left to the pipeline, make migrate-lint judges the staged migrations", stagedAny(migration)},
		{"openapi-breaking", "api", "left to the pipeline, make openapi-breaking judges the staged contract", stagedAny(contract)},
		{"generate-check", goCapability, "left to the pipeline, make generate-check judges the staged generator sources", generatorSources},
	}
}

// leftToPipeline names each step of pipelineSteps whose capability holds
// and whose sources the commit stages; it prints nothing otherwise, because
// a step that does not apply is no news to the person committing.
func (p *precommit) leftToPipeline() Outcome {
	files := p.scan.Index.Staged()
	staged := make([]string, 0, len(files))
	for _, f := range files {
		staged = append(staged, f.Path)
	}
	out := Outcome{}
	for _, s := range pipelineSteps() {
		if p.inForce.Has(s.capability) && s.applies(p.scan.Index.Any, staged) {
			out = join(out, passed(s.name, s.text))
		}
	}
	return out
}

// stagedAny applies a step when a staged path matches its sources.
func stagedAny(source func(string) bool) func(func(func(string) bool) bool, []string) bool {
	return func(_ func(func(string) bool) bool, staged []string) bool { return slices.ContainsFunc(staged, source) }
}

// migration is a SQL file under a directory named migrations, at any depth.
func migration(p string) bool {
	return strings.HasSuffix(p, ".sql") && slices.Contains(strings.Split(path.Dir(p), "/"), "migrations")
}

// contract is an OpenAPI document named openapi, at any depth.
func contract(p string) bool {
	return slices.Contains([]string{"openapi.yaml", "openapi.yml", "openapi.json"}, path.Base(p))
}

// generator is a generator configuration the index tracks and the sources
// it generates from.
type generator struct {
	config, source func(string) bool
}

// generators are mockery over Go interfaces, sqlc over SQL and oapi-codegen
// over the contract, each source set holding its configuration too.
func generators() []generator {
	return []generator{
		{func(p string) bool { return p == mockeryConfig }, func(p string) bool {
			return p == mockeryConfig || (strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go"))
		}},
		{func(p string) bool { return p == sqlcConfig }, func(p string) bool { return p == sqlcConfig || strings.HasSuffix(p, ".sql") }},
		{codegen, func(p string) bool { return codegen(p) || contract(p) }},
	}
}

// codegen is a file under the oapi-codegen configurations.
func codegen(p string) bool { return strings.HasPrefix(p, codegenDir) }

// generatorSources applies generate-check when the commit stages a source
// of a generator whose configuration the index tracks.
func generatorSources(tracked func(func(string) bool) bool, staged []string) bool {
	for _, g := range generators() {
		if tracked(g.config) && slices.ContainsFunc(staged, g.source) {
			return true
		}
	}
	return false
}

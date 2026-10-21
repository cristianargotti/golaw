package rung_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/rung"
	"github.com/cristianargotti/golaw/internal/runnertest"
)

// The lines of the three steps the rung leaves to the pipeline (D13).
const (
	migrateLine  = "golaw pre-commit: migrate-lint: left to the pipeline, make migrate-lint judges the staged migrations"
	openapiLine  = "golaw pre-commit: openapi-breaking: left to the pipeline, make openapi-breaking judges the staged contract"
	generateLine = "golaw pre-commit: generate-check: left to the pipeline, make generate-check judges the staged generator sources"
)

// leftLines are the lines of an outcome that name the three steps.
func leftLines(out rung.Outcome) []string {
	var lines []string
	for _, line := range out.Lines {
		for _, step := range []string{"migrate-lint: ", "openapi-breaking: ", "generate-check: "} {
			if strings.HasPrefix(line, "golaw pre-commit: "+step) {
				lines = append(lines, line)
			}
		}
	}
	return lines
}

// leftCase is a commit and the lines the rung must leave to the pipeline.
type leftCase struct {
	capabilities string
	tracked      map[string]string
	staged       map[string]string
	want         []string
}

func leftCases() map[string]leftCase {
	migration := map[string]string{"db/migrations/0001_init.up.sql": "create table a (id int);\n"}
	contract := map[string]string{"api/openapi.yaml": "openapi: 3.1.0\n"}
	source := map[string]string{"a/a.go": vetClean}
	return map[string]leftCase{
		"a migration under db":        {"[db, go]", nil, migration, []string{migrateLine}},
		"a migration without db":      {"[go]", nil, migration, nil},
		"a query is no migration":     {"[db, go]", nil, map[string]string{"queries/a.sql": "select 1;\n"}, nil},
		"the contract under api":      {"[api, go]", nil, contract, []string{openapiLine}},
		"the contract without api":    {"[go]", nil, contract, nil},
		"another document of the api": {"[api, go]", nil, map[string]string{"api/notes.yaml": "a: 1\n"}, nil},
		"a Go source under mockery":   {"[go]", map[string]string{".mockery.yaml": "all: true\n"}, source, []string{generateLine}},
		"a Go source, no generator":   {"[go]", nil, source, nil},
		"a test file under mockery":   {"[go]", map[string]string{".mockery.yaml": "all: true\n"}, map[string]string{"a/a_test.go": "package a\n"}, nil},
		"a document under mockery":    {"[go]", map[string]string{".mockery.yaml": "all: true\n"}, map[string]string{"docs/a.md": "why\n"}, nil},
		"a query under sqlc":          {"[db, go]", map[string]string{"sqlc.yaml": "version: \"2\"\n"}, map[string]string{"queries/a.sql": "select 1;\n"}, []string{generateLine}},
		"the contract under codegen":  {"[api, go]", map[string]string{"api/codegen/types.yaml": "package: api\n"}, contract, []string{openapiLine, generateLine}},
		"the mockery file itself":     {"[go]", nil, map[string]string{".mockery.yaml": "all: true\n"}, []string{generateLine}},
		"a generator with go off":     {"[image]", map[string]string{".mockery.yaml": "all: true\n"}, source, nil},
		"every step in order":         {"[api, db, go]", map[string]string{"sqlc.yaml": "version: \"2\"\n"}, map[string]string{"db/migrations/0002.sql": "select 1;\n", "api/openapi.json": "{}\n"}, []string{migrateLine, openapiLine, generateLine}},
	}
}

//nolint:paralleltest // the fake tools are found through PATH
func TestPreCommitLeavesTheStepsOfTheM3TemplatesToThePipeline(t *testing.T) {
	fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks, "golangci-lint": linter})
	for name, tc := range leftCases() {
		r := armedRepo(t, "golaw: 1\nkit: {version: 0.1.0}\ncapabilities: "+tc.capabilities+"\n")
		if tc.tracked != nil {
			r.Commit("build: add the generator", tc.tracked)
		}
		r.Stage(tc.staged)
		out := rung.RunPreCommit(t.Context(), deps(t, r))
		if got := leftLines(out); !slices.Equal(got, tc.want) || out.Code != 0 {
			t.Errorf("%s: lines %q, code %d, want %q\n%+v", name, got, out.Code, tc.want, out)
		}
	}
}

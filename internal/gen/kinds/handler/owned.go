package handler

import (
	"path"
	"slices"
	"strconv"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/genheader"
)

// opsPerFile keeps an owned file of 24 methods of six lines and its red test
// file of 24 tests of eight lines under the 300 line limit with their
// imports, so a tag splits into numbered files before either passes it.
const opsPerFile = 24

// group is the operations of one tag, which share one owned file stem.
type group struct {
	Stem string
	Ops  []operation
}

// groups keeps the stems in the order of each one's first operation, so the
// owned files follow the document an agent reads.
func groups(ops []operation) []group {
	var out []group
	at := map[string]int{}
	for _, op := range ops {
		i, ok := at[op.Stem]
		if !ok {
			i = len(out)
			at[op.Stem] = i
			out = append(out, group{Stem: op.Stem})
		}
		out[i].Ops = append(out[i].Ops, op)
	}
	return out
}

// chunkPaths names chunk n of a stem: the first one unnumbered, so a small
// document reads plans.go, and the next ones numbered from 2. It answers
// the code path, then the path of its red test file.
func chunkPaths(dir, stem string, n int) (string, string) {
	name := stem
	if n > 1 {
		name += "_" + strconv.Itoa(n)
	}
	return path.Join(dir, name+".go"), path.Join(dir, name+"_test.go")
}

func codeImports(v view) []genkind.Import {
	return append(std(stdContext), v.APIGen, v.Support)
}

func testImports(v view) []genkind.Import {
	return append(std(stdErrors, stdTesting), v.APIGen, v.Support)
}

// operationSpecs are the two files of one chunk: the methods with their
// holes and the red test of each hole.
func operationSpecs(v view, stem string, n int, ops []operation) []spec {
	code, test := chunkPaths(v.Dir, stem, n)
	return []spec{codeSpec(v, code, ops), testSpec(v, test, ops)}
}

// codeSpec and testSpec render one side of a chunk alone, so reconcile can
// write again an owned file the agent deleted without touching its pair.
func codeSpec(v view, file string, ops []operation) spec {
	p := pageOf(v)
	p.Ops = ops
	return spec{"operations.go.tmpl", file, p, codeImports(v)}
}

func testSpec(v view, file string, ops []operation) spec {
	p := pageOf(v)
	p.Ops = ops
	return spec{"operations_test.go.tmpl", file, p, testImports(v)}
}

// owned renders the files golaw new writes once and never rewrites, so they
// carry no golaw header (M7.4.2): every operation chunk with its red tests,
// then the status rules with theirs.
func owned(v view) ([]genkind.File, error) {
	var specs []spec
	for _, g := range groups(v.Ops) {
		for i, part := range chunk(g.Ops, opsPerFile) {
			specs = append(specs, operationSpecs(v, g.Stem, i+1, part)...)
		}
	}
	p := pageOf(v)
	specs = slices.Concat(specs, []spec{
		{"status.go.tmpl", path.Join(v.Dir, "status.go"), p, nil},
		{"status_internal_test.go.tmpl", path.Join(v.Dir, "status_internal_test.go"), p, std(stdTesting)},
	})
	return renderAll(v.Module, specs, genheader.Header{})
}

package usecase_test

import (
	"go/parser"
	"go/token"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/goldentest"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/gentest"
	"github.com/cristianargotti/golaw/internal/model"
)

const (
	module = "example.com/slice"
	area   = "internal/application/growth/"
)

var asked = genkind.Name{Area: "growth", Type: "ReadPlan"}

func input(t *testing.T, files map[string][]byte, entries ...manifest.Entry) genkind.Input {
	t.Helper()
	return genkind.Input{
		Module: module, Version: "0.3.0", Policy: model.DefaultGenerate(), Snapshot: gentest.Snapshot(t, files),
		Manifest:  manifest.Manifest{Schema: manifest.Schema, Support: manifest.Support{Path: "internal/gensupport", Template: "support@1"}, Entries: entries},
		Contracts: gentest.FakeContracts{},
	}
}

func readPlan(ports ...string) genkind.Request {
	return genkind.Request{Name: asked, Ports: ports, In: "AccountID", Out: "Plan"}
}

// entry is the manifest entry of a use case of growth whose snake case
// name is given, over AccountID and Plan.
func entry(typ, snake string, ports ...string) manifest.Entry {
	return manifest.Entry{
		Kind: genkind.KindUsecase, Template: "usecase@1", Name: "growth." + typ, Ports: ports, In: "AccountID", Out: "Plan",
		Regenerable: []string{area + "zz_generated_" + snake + ".go", area + "zz_generated_usecases.go", area + "zz_generated_usecases_test.go"},
		Owned:       []string{area + snake + ".go", area + snake + "_test.go"},
	}
}

//nolint:unparam // the name is the port each row enrolls, so a row reads which port has its entry
func portEntry(name string) manifest.Entry {
	dir := area + strings.ToLower(name) + "test/"
	return manifest.Entry{
		Kind: genkind.KindPort, Template: "port@1", Name: "growth." + name,
		Contracts:   []manifest.Contract{{Path: area + "ports.go", Symbol: name}},
		Regenerable: []string{dir + "zz_generated_fake.go", dir + "zz_generated_suite.go", dir + "zz_generated_suite_test.go"},
		Owned:       []string{dir + "cases.go"},
	}
}

func paths(files []genkind.File) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, f.Path)
	}
	return out
}

func content(t *testing.T, files []genkind.File, path string) string {
	t.Helper()
	for _, f := range files {
		if f.Path == path {
			return string(f.Content)
		}
	}
	t.Fatalf("no file %s among %v", path, paths(files))
	return ""
}

func assertHas(t *testing.T, text string, wants ...string) {
	t.Helper()
	normal := strings.Join(strings.Fields(text), " ")
	for _, want := range wants {
		if !strings.Contains(normal, strings.Join(strings.Fields(want), " ")) {
			t.Errorf("text lacks %q:\n%s", want, text)
		}
	}
}

func importPaths(t *testing.T, text string) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "x.go", text, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(f.Imports))
	for _, spec := range f.Imports {
		out = append(out, strings.Trim(spec.Path.Value, `"`))
	}
	return out
}

func assertLaws(t *testing.T, out genkind.Output) {
	t.Helper()
	files := map[string][]byte{}
	for _, f := range slices.Concat(out.Regenerable, out.Owned) {
		files[f.Path] = f.Content
	}
	for _, law := range goldentest.Laws() {
		for _, violation := range law.Assert(files) {
			t.Errorf("%s: %s", law.Name, violation)
		}
	}
}

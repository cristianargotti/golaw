package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/thirdparty"
	"github.com/cristianargotti/golaw/internal/runnertest"
)

// oapiGroup is the three codegen configurations of one specification, each
// writing one file of the same output directory.
func oapiGroup() []thirdparty.Generator {
	gens := make([]thirdparty.Generator, 0, 3)
	for _, config := range []string{"api/server.cfg.yaml", "api/spec.cfg.yaml", "api/types.cfg.yaml"} {
		gens = append(gens, thirdparty.Generator{
			Name: "oapi-codegen", Config: config, Args: []string{"-config", config, "api/openapi.yaml"},
			Inputs: []string{"api/openapi.yaml"}, Outputs: []string{"api/gen"},
		})
	}
	return gens
}

func TestDriftGroupsJoinTheConfigurationsOfOneOutput(t *testing.T) {
	t.Parallel()
	sqlc := thirdparty.Generator{Name: "sqlc", Config: "sqlc.yaml", Outputs: []string{"internal/adapters/out/postgres/sqlcgen"}}
	gens := append([]thirdparty.Generator{sqlc}, oapiGroup()...)
	groups := driftGroups(gens)
	if len(groups) != 2 || !reflect.DeepEqual(groups[0], []thirdparty.Generator{sqlc}) || !reflect.DeepEqual(groups[1], oapiGroup()) {
		t.Fatalf("groups = %+v", groups)
	}
	other := thirdparty.Generator{Name: "oapi-codegen", Config: "billing/cfg.yaml", Outputs: []string{"billing/gen"}}
	if groups := driftGroups(append(oapiGroup(), other)); len(groups) != 2 || len(groups[1]) != 1 {
		t.Fatalf("a configuration of another output joined the group: %+v", groups)
	}
}

// A configuration never reports the file a sibling of its output writes:
// the group runs into one stage, so each checkout file is judged once.
func TestThirdPartyDriftJudgesASharedOutputOnce(t *testing.T) {
	t.Parallel()
	ta, root, _ := fakeGenGo(t, runnertest.Behavior{Stdout: "ok"})
	for _, name := range []string{"server", "spec", "types"} {
		writeRepoFile(t, root, "api/"+name+".cfg.yaml", "package: gen\n")
		writeRepoFile(t, root, "api/gen/"+name+".gen.go", "package gen\n")
	}
	writeRepoFile(t, root, "api/openapi.yaml", "openapi: 3.0.3\n")
	run, err := ta.prepareGen(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	drift, unmeasured, err := ta.bind().thirdPartyDrift(t.Context(), root, run.snapshot, oapiGroup())
	if err != nil || len(unmeasured) != 0 {
		t.Fatalf("drift = %+v, unmeasured %q, %v", drift, unmeasured, err)
	}
	paths := make([]string, 0, len(drift))
	for _, d := range drift {
		paths = append(paths, d.Path)
	}
	if want := []string{"api/gen/server.gen.go", "api/gen/spec.gen.go", "api/gen/types.gen.go"}; !reflect.DeepEqual(paths, want) {
		t.Fatalf("drift paths = %q, want each once %q", paths, want)
	}
}

// missingGo binds a go tool that cannot provide any generator.
func missingGo(t *testing.T, ta testApp) testApp {
	t.Helper()
	bound := ta.bind()
	bin := t.TempDir()
	runnertest.Install(t, bin, "go", runnertest.Behavior{Stderr: "go: missing go.sum entry", Code: 1})
	bound.environ = func() []string { return []string{"PATH=" + bin} }
	ta.bind = func() binding { return bound }
	return ta
}

// Red row: the three configurations of one output are one unmeasured line.
// Near row: a generator whose inputs the checkout lacks, and whose output
// directory holds nothing, does not apply, so a repository that never
// opted into it is neither unmeasured nor red.
func TestGenCheckNamesEachOutputGroupOnceAndSkipsAnUnusedGenerator(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		inputs     bool
		unmeasured int
	}{
		"red: inputs present": {inputs: true, unmeasured: 1},
		"near: inputs absent": {inputs: false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ta, root := newPort(t)
			ta = missingGo(t, ta)
			writeRepoFile(t, root, "tools/go.mod", "module example.com/slice/tools\n\ngo 1.27\n")
			for _, config := range []string{"server", "spec", "types"} {
				writeRepoFile(t, root, "api/codegen/"+config+".yaml", "package: apigen\n")
			}
			if !tc.inputs {
				removeContracts(t, root)
			}
			code := ta.run(t.Context(), []string{"gen", "--check"})
			got := strings.Count(ta.stdout.String(), "unmeasured oapi-codegen")
			if got != tc.unmeasured || (tc.unmeasured == 0) != (code != exitFailure) {
				t.Errorf("exit %d, %d oapi-codegen lines, want %d:\n%s", code, got, tc.unmeasured, ta.stdout.String())
			}
		})
	}
}

// removeContracts drops the third party contracts and outputs of root.
func removeContracts(t *testing.T, root string) {
	t.Helper()
	for _, dir := range []string{"api/gen", "api/openapi.yaml", "db", "internal/adapters/out/postgres/sqlcgen"} {
		if err := os.RemoveAll(filepath.Join(root, filepath.FromSlash(dir))); err != nil {
			t.Fatal(err)
		}
	}
}

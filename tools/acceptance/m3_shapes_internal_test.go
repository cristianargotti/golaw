package main

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw"
	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
	"github.com/cristianargotti/golaw/internal/runner"
)

// m3Shape dispatches one check alone over a tree in scan mode, as golaw
// scan dispatches it, under the capabilities and the options of the check.
func m3Shape(t *testing.T, c model.Check, files map[string][]byte, capabilities []string, options model.Options) model.Result {
	t.Helper()
	builder := engine.New()
	if err := builder.Register(c); err != nil {
		t.Fatal(err)
	}
	registry, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	policy := model.CoreDefaults()
	policy.Capabilities = capabilities
	if options != nil {
		policy.Options = map[string]model.Options{c.Spec().ID: options}
	}
	tree := proof.NewTree(files)
	run, err := engine.Dispatch(t.Context(), registry, model.Input{
		Mode: model.ModeScan, Policy: &policy, Snapshot: tree, Targets: tree.Files(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Outcomes) != 1 {
		t.Fatalf("outcomes = %d", len(run.Outcomes))
	}
	return run.Outcomes[0].Result
}

// m3BirthTree is the birth fixture as init --apply leaves it: every core
// template beside it, a destination the birth holds kept as born (init only
// proposes there), and nothing under .github/, which its CODEOWNERS gives
// to another team.
func m3BirthTree(t *testing.T, capabilities []string) map[string][]byte {
	t.Helper()
	files, err := m3BirthFiles(testRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	tools := make(map[string]string)
	for _, tool := range runner.Toolchain() {
		tools[tool.Name] = tool.Version
	}
	rendered, err := assets.RenderAllContext(golaw.FS, assets.Context{
		ModulePath: "example.test/birth", GoVersion: "1.27", KitVersion: "0.3.0", Language: "pt-BR",
		Capabilities: capabilities, Decisions: "docs/adr", Main: "root", Tools: tools, CIBackend: "none",
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := range rendered {
		if _, held := files[rendered[i].Path]; held || strings.HasPrefix(rendered[i].Path, ".github/") {
			continue
		}
		files[rendered[i].Path] = rendered[i].Data
	}
	return files
}

// m3Renamed renames the two targets of the birth Makefile that golaw.mk
// defines too, the repair init names.
func m3Renamed(makefile string) string {
	return strings.ReplaceAll(strings.ReplaceAll(makefile, "\nlint:", "\nlint-template:"), "\ntest:", "\ntest-compose:")
}

// m3Included is a Makefile with the include line init proposes accepted.
func m3Included(makefile string) string {
	return makefile + "include golaw.mk\n"
}

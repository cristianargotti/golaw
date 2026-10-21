package main

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw"
	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/runner"
)

// coreAssets writes beside the module of repository every file golaw init
// renders for it under the capabilities its policy names, go when the root
// holds no policy, but the policy and the module file repository already holds:
// a scan that expects a clean repository judges the tree init leaves, so a
// check that joins the catalog meets the kit's own templates there, the
// targets and files of each capability in force included, and never an
// empty repository.
func coreAssets(t *testing.T, root string) {
	t.Helper()
	capabilities := policyCapabilities(t, root)
	tools := make(map[string]string)
	for _, tool := range runner.Toolchain() {
		tools[tool.Name] = tool.Version
	}
	rendered, err := assets.RenderAllContext(golaw.FS, assets.Context{
		CIBackend: "none", ModulePath: "example.test/m", GoVersion: "1.27", KitVersion: testVersion, Language: model.DefaultLanguage,
		Capabilities: capabilities, Decisions: "docs/adr", Main: bootstrap.MainRoot, Tools: tools,
	})
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for i := range rendered {
		if rendered[i].Path != "golaw.yml" && rendered[i].Path != "go.mod" {
			files[rendered[i].Path] = string(rendered[i].Data)
		}
	}
	writeFiles(t, root, files)
}

// policyCapabilities answers the capabilities of the policy at root, as init
// reads them before it renders, and go when the root holds no policy.
func policyCapabilities(t *testing.T, root string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, policy.FileName))
	if errors.Is(err, fs.ErrNotExist) {
		return []string{"go"}
	}
	if err != nil {
		t.Fatal(err)
	}
	f, err := policy.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	return f.Capabilities
}

// TestCoreAssetsWriteTheTreeInitLeaves pins what coreAssets writes: the
// Makefile with its include, golaw.mk, the lint configuration and the
// lefthook file land beside the module, while the policy and the module
// file stay as repository wrote them.
func TestCoreAssetsWriteTheTreeInitLeaves(t *testing.T) {
	t.Parallel()
	root := repository(t, true)
	before, err := os.ReadFile(filepath.Join(root, "golaw.yml"))
	if err != nil {
		t.Fatal(err)
	}
	coreAssets(t, root)
	for _, path := range []string{"Makefile", "golaw.mk", ".golangci.yml", "lefthook.yml"} {
		if _, statErr := os.Stat(filepath.Join(root, path)); statErr != nil {
			t.Errorf("%s: %v", path, statErr)
		}
	}
	module, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil || string(module) != "module example.test/m\n\ngo 1.27\n" {
		t.Fatalf("go.mod = %q, %v", module, err)
	}
	after, err := os.ReadFile(filepath.Join(root, "golaw.yml"))
	if err != nil || !bytes.Equal(after, before) {
		t.Fatalf("golaw.yml changed: %v", err)
	}
}

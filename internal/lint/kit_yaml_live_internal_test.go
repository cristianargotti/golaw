package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw"
	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/runner"
)

// kitSurfaceLine is the line init --surface-rules adds to golaw.yml.
const kitSurfaceLine = "surface: { rules: strict }\n"

// TestTheKitsOwnYAMLIsCleanUnderTheStrictSet renders every template with
// every capability and the policy file init writes, and lints each YAML
// file under the strict set init selects: an adopter is never red on day
// one on files the kit wrote. A machine without yamllint skips.
func TestTheKitsOwnYAMLIsCleanUnderTheStrictSet(t *testing.T) {
	t.Parallel()
	tools := make(map[string]string)
	for _, tool := range runner.Toolchain() {
		tools[tool.Name] = tool.Version
	}
	rendered, err := assets.RenderAllContext(golaw.FS, assets.Context{
		CIBackend: "none", ModulePath: "example.test/service", GoVersion: "1.27", KitVersion: "1.2.3", Language: model.DefaultLanguage,
		Capabilities: model.Capabilities(), Decisions: "docs/adr", Main: bootstrap.MainRoot, Tools: tools,
	})
	if err != nil {
		t.Fatal(err)
	}
	policy, err := bootstrap.Render(bootstrap.Inputs{
		KitVersion: "1.2.3", Language: model.DefaultLanguage, Capabilities: model.Capabilities(),
		Layout: bootstrap.Layout{Main: bootstrap.MainRoot, Decisions: "docs/adr", Go: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	root := pythonRoot(t)
	files := []string{"golaw.yml"}
	writeKitFile(t, root, "golaw.yml", append(policy, kitSurfaceLine...))
	for _, r := range rendered {
		if strings.HasSuffix(r.Path, ".yml") || strings.HasSuffix(r.Path, ".yaml") {
			files = append(files, r.Path)
			writeKitFile(t, root, r.Path, r.Data)
		}
	}
	for _, issue := range runPythonDriver(t, yamllintDriver(), model.SurfaceStrict, root, files) {
		t.Errorf("%s:%d %s/%s %s", issue.Path, issue.Line, issue.Tool, issue.Rule, issue.Message)
	}
}

// writeKitFile writes data at the slash path rel under root.
func writeKitFile(t *testing.T, root, rel string, data []byte) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), pythonDirMode); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, pythonFileMode); err != nil {
		t.Fatal(err)
	}
}

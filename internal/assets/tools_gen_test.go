package assets_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw"
	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
	"github.com/cristianargotti/golaw/internal/runner"
)

func TestTheToolsModulePinsEachGeneratorUnderItsCapability(t *testing.T) {
	t.Parallel()
	modules := map[string]string{
		"goverter": "github.com/jmattheis/goverter", "oapi-codegen": "github.com/oapi-codegen/oapi-codegen/v2", "sqlc": "github.com/sqlc-dev/sqlc",
	}
	tests := map[string]struct {
		capabilities []string
		want         []string
	}{
		"red: every generator capability":       {capabilities: []string{"api", "db", "go", "hexagonal"}, want: []string{"goverter", "oapi-codegen", "sqlc"}},
		"red: a database only":                  {capabilities: []string{"db", "go"}, want: []string{"sqlc"}},
		"red: an HTTP contract only":            {capabilities: []string{"api", "go"}, want: []string{"oapi-codegen"}},
		"near: the core downloads no generator": {capabilities: []string{"go"}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			context := assetContext()
			context.Capabilities = tc.capabilities
			text := string(renderedAsset(t, "tools/go.mod", context).Data)
			for _, tool := range runner.Generators() {
				wanted := strings.Contains(strings.Join(tc.want, " "), tool.Name)
				directive := strings.Contains(text, "\t"+tool.Module+"\n")
				require := strings.Contains(text, "\t"+modules[tool.Name]+" v"+tool.Version+"\n")
				if directive != wanted || require != wanted {
					t.Errorf("%s: tool line %v, require line %v, want %v in\n%s", tool.Name, directive, require, wanted, text)
				}
			}
			requireToolsModule(t, text, "1.27")
		})
	}
}

// TestTheBaselineRecordsEachRenderedGenerator keeps the dependency records
// check silent over a fresh init with every generator capability: the
// tools module requires the generators, so the baseline record init writes
// must name them, or the kit's own files would leave the pipeline red.
func TestTheBaselineRecordsEachRenderedGenerator(t *testing.T) {
	t.Parallel()
	for name, capabilities := range map[string][]string{
		"red: every generator capability":     {"api", "db", "go", "hexagonal"},
		"near: the core records no generator": {"go"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			context := assetContext()
			context.Capabilities = capabilities
			files := renderedTree(t, context)
			tree := proof.NewTree(files)
			result, err := policyfiles.DependencyRecords().Run(t.Context(), model.Input{
				Mode: model.ModeScan, Policy: proof.Policy(), Snapshot: tree, Targets: tree.Files(),
			})
			if err != nil || result.Status.Kind != model.StatusMeasured || len(result.Findings) != 0 {
				t.Fatalf("records over %v = %+v, error %v", capabilities, result, err)
			}
			record, tools := string(files["docs/adr/0001-inherited-baseline.md"]), string(files["tools/go.mod"])
			requireRuntimes(t, record, strings.Contains(strings.Join(capabilities, " "), "api"))
			for _, tool := range runner.Generators() {
				if strings.Contains(record, "| v"+tool.Version+" | kit tool |") != strings.Contains(tools, tool.Module) {
					t.Errorf("the baseline table and the tools module disagree on %s", tool.Name)
				}
			}
		})
	}
}

// requireRuntimes holds the baseline to the modules the generated handler
// imports: listed when the api capability renders the generator, else not.
func requireRuntimes(t *testing.T, record string, api bool) {
	t.Helper()
	for _, runtime := range []string{"github.com/getkin/kin-openapi", "github.com/oapi-codegen/runtime"} {
		if strings.Contains(record, "\n  - "+runtime+"\n") != api {
			t.Errorf("the baseline lists %s = %v, want %v: the generated handler imports it", runtime, !api, api)
		}
	}
}

// renderedTree renders every asset of the context into a tree keyed by
// path, with the module line init keeps from the person's go.mod.
func renderedTree(t *testing.T, context assets.Context) map[string][]byte {
	t.Helper()
	rendered, err := assets.RenderAllContext(golaw.FS, context)
	if err != nil {
		t.Fatal(err)
	}
	files := make(map[string][]byte)
	for i := range rendered {
		files[rendered[i].Path] = rendered[i].Data
	}
	files["go.mod"] = append([]byte("module example.test/service\n"), files["go.mod"]...)
	return files
}

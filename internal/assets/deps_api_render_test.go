package assets_test

import (
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

func TestAPIGenerationRemainsSeparate(t *testing.T) {
	t.Parallel()
	context := assetContext()
	context.Capabilities = []string{"go", "api"}
	for _, tc := range []struct {
		name     string
		generate map[string]bool
	}{
		{"types", map[string]bool{"models": true}},
		{"server", map[string]bool{"std-http-server": true, "strict-server": true}},
		{"spec", map[string]bool{"embedded-spec": true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var config struct {
				Package  string          `yaml:"package"`
				Generate map[string]bool `yaml:"generate"`
				Output   string          `yaml:"output"`
			}
			data := renderedAsset(t, "api/codegen/"+tc.name+".yaml", context).Data
			if err := yaml.Unmarshal(data, &config); err != nil {
				t.Fatal(err)
			}
			if config.Package != "api" || config.Output != "api/generated/"+tc.name+".gen.go" ||
				!reflect.DeepEqual(config.Generate, tc.generate) {
				t.Fatalf("generation configuration = %+v", config)
			}
		})
	}
}

func TestAPIRedoclyTemplatePassesPolicy(t *testing.T) {
	t.Parallel()
	context := assetContext()
	context.Capabilities = []string{"go", "api"}
	file := renderedAsset(t, "redocly.yaml", context)
	for _, tc := range []struct{ name, source string }{
		{"before source exists", ""},
		{"source exists", "openapi: 3.0.3\npaths: {}\ncomponents:\n  schemas:\n" +
			"    Amount: {type: number, format: double}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			files := map[string][]byte{file.Path: file.Data}
			if tc.source != "" {
				files["api/openapi.yaml"] = []byte(tc.source)
			}
			tree := proof.NewTree(files)
			result, err := policyfiles.OpenAPIAssertions().Run(t.Context(), model.Input{
				Mode: model.ModeScan, Policy: proof.Policy(), Snapshot: tree, Targets: tree.Files(),
			})
			if err != nil || result.Status.Kind != model.StatusMeasured || len(result.Findings) != 0 {
				t.Fatalf("rendered Redocly configuration = %+v, error %v", result, err)
			}
		})
	}
}

func TestAPIServiceTemplatePassesPolicy(t *testing.T) {
	t.Parallel()
	capabilities := []string{"api", "go"}
	tree := proof.NewTree(serviceFiles(t, capabilities))
	result, err := policyfiles.OpenAPIAssertions().Run(t.Context(), model.Input{
		Mode: model.ModeScan, Policy: servicePolicy(t, capabilities), Snapshot: tree, Targets: tree.Files(),
	})
	if err != nil || result.Status.Kind != model.StatusMeasured || len(result.Findings) != 0 {
		t.Fatalf("rendered API service = %+v, error %v", result, err)
	}
}

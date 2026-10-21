package assets_test

import (
	"path"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/checks/paths"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

func TestRenderedGeneratorOutputsAreProtected(t *testing.T) {
	t.Parallel()
	context := assetContext()
	context.Capabilities = []string{"go", "api", "db"}
	for _, name := range []string{
		"api/codegen/types.yaml", "api/codegen/server.yaml", "api/codegen/spec.yaml",
	} {
		file := renderedAsset(t, name, context)
		var config struct {
			Output string         `yaml:"output"`
			Rest   map[string]any `yaml:",inline"`
		}
		if err := yaml.Unmarshal(file.Data, &config); err != nil {
			t.Fatal(err)
		}
		if config.Output == "" {
			t.Fatalf("asset %s has no output", name)
		}
		assertGeneratedDirectory(t, path.Dir(config.Output))
	}
	database := renderedAsset(t, "sqlc.yaml", context)
	var config struct {
		SQL []struct {
			Gen struct {
				Go struct {
					Out string `yaml:"out"`
				} `yaml:"go"`
			} `yaml:"gen"`
		} `yaml:"sql"`
	}
	if err := yaml.Unmarshal(database.Data, &config); err != nil {
		t.Fatal(err)
	}
	if len(config.SQL) != 1 || config.SQL[0].Gen.Go.Out == "" {
		t.Fatalf("database output = %+v", config)
	}
	assertGeneratedDirectory(t, config.SQL[0].Gen.Go.Out)
}

func assertGeneratedDirectory(t *testing.T, directory string) {
	t.Helper()
	tree := proof.NewTree(map[string][]byte{
		".gitignore": []byte(strings.TrimSuffix(directory, "/") + "/\n"),
	})
	got, err := paths.GeneratedTracked().Run(t.Context(), model.Input{
		Mode: model.ModeScan, Policy: proof.Policy(), Snapshot: tree, Targets: tree.Files(),
	})
	if err != nil || len(got.Findings) != 1 || got.Findings[0].Code != "generated-ignored" {
		t.Fatalf("directory %s is not protected: %+v, %v", directory, got, err)
	}
}

package assets_test

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw"
	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/source"
)

// codegenNames are the three oapi-codegen configurations the api capability renders.
func codegenNames() []string { return []string{"server", "spec", "types"} }

func TestCodegenOutputFollowsTheGenerateBlock(t *testing.T) {
	t.Parallel()
	declared := model.DefaultGenerate()
	declared.ThirdParty.OpenAPIOut = "internal/api/gen"
	tests := map[string]struct {
		generate model.GeneratePolicy
		dir      string
	}{
		"red: the default output is api/gen":                 {generate: model.DefaultGenerate(), dir: "api/gen"},
		"red: a declared output is followed":                 {generate: declared, dir: "internal/api/gen"},
		"near: values without a generate block keep api/gen": {dir: "api/gen"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			context := assetContext()
			context.Capabilities = []string{"api", "go"}
			context.Generate = tc.generate
			for _, codegen := range codegenNames() {
				assertCodegenOutput(t, context, codegen, tc.dir)
			}
		})
	}
}

// assertCodegenOutput keeps the table test under the cognitive limit of the lint.
func assertCodegenOutput(t *testing.T, context assets.Context, codegen, dir string) {
	t.Helper()
	var config struct {
		Package string `yaml:"package"`
		Output  string `yaml:"output"`
	}
	data := renderedAsset(t, "api/codegen/"+codegen+".yaml", context).Data
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	if config.Package != "apigen" || config.Output != dir+"/"+codegen+".gen.go" {
		t.Errorf("%s: package %q, output %q, want apigen under %s", codegen, config.Package, config.Output, dir)
	}
	if strings.Contains(string(data), "api/generated") {
		t.Errorf("%s still names api/generated:\n%s", codegen, data)
	}
}

func TestServiceExampleKeepsItsAPIInPackageApigen(t *testing.T) {
	t.Parallel()
	files, err := assets.RenderService(golaw.FS, templateValues())
	if err != nil {
		t.Fatal(err)
	}
	byPath := make(map[string][]byte, len(files))
	for i := range files {
		byPath[files[i].Path] = files[i].Data
		if strings.HasPrefix(files[i].Path, "api/generated/") {
			t.Errorf("the example still renders %s", files[i].Path)
		}
	}
	for _, codegen := range codegenNames() {
		path := "api/gen/" + codegen + ".gen.go"
		data, ok := byPath[path]
		if !ok {
			t.Errorf("the example renders no %s", path)
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, data, parser.PackageClauseOnly)
		if err != nil || file.Name.Name != "apigen" {
			t.Errorf("%s: package %v, error %v, want apigen", path, file, err)
		}
		if got := source.Classify(path, data, true); got != model.ClassGenerated {
			t.Errorf("%s is classed %v, want the generated class of its oapi-codegen header", path, got)
		}
	}
}

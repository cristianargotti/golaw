package main

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw"
	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestM3DependabotUnderAForeignGithubIsNotApplicable(t *testing.T) {
	t.Parallel()
	absent := m3Shape(t, policyfiles.DependabotCooldown(), m3BirthTree(t, []string{"go"}), []string{"go"}, nil)
	if absent.Status.Kind != model.StatusNotApplicable ||
		!strings.Contains(absent.Status.Reason, "CODEOWNERS") || !strings.Contains(absent.Status.Reason, ".github/") {
		t.Fatalf("absent under a foreign owner = %+v", absent.Status)
	}
	present := m3BirthTree(t, []string{"go"})
	present[".github/dependabot.yml"] = []byte("version: 2\nupdates:\n  - package-ecosystem: gomod\n    directory: /\n" +
		"    schedule:\n      interval: weekly\n    cooldown:\n      default-days: 1\n")
	judged := m3Shape(t, policyfiles.DependabotCooldown(), present, []string{"go"}, nil)
	if judged.Status.Kind != model.StatusMeasured || len(judged.Findings) == 0 {
		t.Fatalf("a present file must be judged as written: %+v, %+v", judged.Status, judged.Findings)
	}
}

func TestM3SwaggerOnlyTreeIsMeasuredCleanUnderApi(t *testing.T) {
	t.Parallel()
	capabilities := []string{"api", "go"}
	birth := m3BirthTree(t, capabilities)
	swagger := m3Shape(t, policyfiles.OpenAPIAssertions(), birth, capabilities, nil)
	if swagger.Status.Kind != model.StatusMeasured || len(swagger.Findings) != 0 {
		t.Fatalf("Swagger 2.0 beside a contract root that does not exist yet = %+v, %+v", swagger.Status, swagger.Findings)
	}
	service, err := assets.RenderService(golaw.FS, assets.Values{ModulePath: "example.test/birth", GoVersion: "1.27"})
	if err != nil {
		t.Fatal(err)
	}
	for i := range service {
		if service[i].Path == "api/openapi.yaml" {
			birth[m3RedoclyRoot(t, birth)] = service[i].Data
		}
	}
	measured := m3Shape(t, policyfiles.OpenAPIAssertions(), birth, capabilities, nil)
	if measured.Status.Kind != model.StatusMeasured {
		t.Fatalf("an OpenAPI 3 contract beside the Swagger tree = %+v", measured.Status)
	}
}

// m3RedoclyRoot answers the contract path the rendered redocly.yaml names.
func m3RedoclyRoot(t *testing.T, files map[string][]byte) string {
	t.Helper()
	var config struct {
		APIs map[string]struct {
			Root string `yaml:"root"`
		} `yaml:"apis"`
	}
	if err := yaml.Unmarshal(files["redocly.yaml"], &config); err != nil {
		t.Fatal(err)
	}
	for _, api := range config.APIs {
		if api.Root != "" {
			return api.Root
		}
	}
	t.Fatal("the rendered redocly.yaml names no contract root")
	return ""
}

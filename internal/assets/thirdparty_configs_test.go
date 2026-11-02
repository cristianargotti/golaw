package assets_test

import (
	"regexp"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/thirdparty"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// TestThePinnedGeneratorsRunOverTheRenderedConfigurations keeps the paths
// thirdparty knows equal to the destinations init writes: a fresh init with
// every generator capability enables sqlc and one oapi-codegen per codegen
// configuration, a change to a contract triggers them, and the managed
// sqlc.yaml reads the directories the sqlc inputs name. goverter stays off
// because init renders no converter.
func TestThePinnedGeneratorsRunOverTheRenderedConfigurations(t *testing.T) {
	t.Parallel()
	context := assetContext()
	context.Capabilities = []string{"api", "db", "go", "hexagonal"}
	files := renderedTree(t, context)
	policy := model.CoreDefaults()
	policy.Capabilities = context.Capabilities
	generators := thirdparty.Generators(&policy, proof.NewTree(files))
	configs := make([]string, 0, len(generators))
	for _, g := range generators {
		configs = append(configs, g.Name+" "+g.Config)
	}
	want := []string{"sqlc sqlc.yaml", "oapi-codegen api/codegen/server.yaml", "oapi-codegen api/codegen/spec.yaml", "oapi-codegen api/codegen/types.yaml"}
	if !slices.Equal(configs, want) {
		t.Fatalf("generators over a fresh init = %v, want %v", configs, want)
	}
	for changed, count := range map[string]int{"db/queries/plans.sql": 1, "api/openapi.yaml": 3, "README.md": 0} {
		if got := len(thirdparty.Triggered(generators, []string{changed})); got != count {
			t.Errorf("a change to %s triggers %d generators, want %d", changed, got, count)
		}
	}
	sqlc := string(files["sqlc.yaml"])
	for key, dir := range map[string]string{"schema": "db/migrations", "queries": "db/queries"} {
		if !regexp.MustCompile(`(?m)^\s+` + key + `: ` + dir + `$`).MatchString(sqlc) {
			t.Errorf("the managed sqlc.yaml does not read %s from %s:\n%s", key, dir, sqlc)
		}
	}
}

// Near row: the core alone renders no generator configuration, so no
// generator runs even under a policy that enables every capability.
func TestThePinnedGeneratorsStayOffOverTheCore(t *testing.T) {
	t.Parallel()
	context := assetContext()
	context.Capabilities = []string{"go"}
	policy := model.CoreDefaults()
	policy.Capabilities = []string{"api", "db", "go", "hexagonal"}
	if got := thirdparty.Generators(&policy, proof.NewTree(renderedTree(t, context))); len(got) != 0 {
		t.Errorf("generators over the core = %+v", got)
	}
}

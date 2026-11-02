package assets_test

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/model"
)

func renderedSQLCOut(t *testing.T, out string) string {
	t.Helper()
	context := assetContext()
	context.Capabilities = []string{"go", "db"}
	context.Generate.ThirdParty.SQLCOut = out
	var config struct {
		SQL []struct {
			Gen struct {
				Go struct {
					Out string `yaml:"out"`
				} `yaml:"go"`
			} `yaml:"gen"`
		} `yaml:"sql"`
	}
	if err := yaml.Unmarshal(renderedAsset(t, "sqlc.yaml", context).Data, &config); err != nil {
		t.Fatal(err)
	}
	if len(config.SQL) != 1 {
		t.Fatalf("sqlc.yaml holds %d sql blocks", len(config.SQL))
	}
	return config.SQL[0].Gen.Go.Out
}

func TestSQLCOutputFollowsTheGenerateBlock(t *testing.T) {
	t.Parallel()
	defaults := model.DefaultGenerate().ThirdParty.SQLCOut
	tests := []struct{ name, out, want string }{
		{"the default layout", defaults, "internal/adapters/out/postgres/sqlcgen"},
		{"a detected layout", "internal/infra/postgres/sqlcgen", "internal/infra/postgres/sqlcgen"},
		{"near: a context without a generate block", "", defaults},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := renderedSQLCOut(t, tc.out); got != tc.want {
				t.Errorf("sqlc out = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSQLCOutputLeavesTheOutboundTree(t *testing.T) {
	t.Parallel()
	if got := renderedSQLCOut(t, ""); strings.Contains(got, "/outbound/") {
		t.Errorf("sqlc out = %q: the kit's fixed outbound path is the drift M7.6.2 removes", got)
	}
}

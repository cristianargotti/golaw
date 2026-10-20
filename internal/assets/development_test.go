package assets_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
)

func TestDevelopmentConfigurations(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ path, want string }{
		{".gremlins.yaml", `unleash:
  workers: 4
  timeout-coefficient: 10
  threshold:
    efficacy: 85
    mutant-coverage: 0
`},
		{".mockery.yaml", `template: testify
dir: test/mocks
pkgname: mocks
force-file-write: true
packages: {}
`},
	} {
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()
			assertAsset(t, tc.path, tc.want, assets.OwnedByKit)
		})
	}
}

func TestInheritedBaselineRecordsEveryDirectRequirement(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, decisions, language, title string
	}{
		{"default", "docs/adr", "pt-BR", "# Dependências herdadas"},
		{"english", "docs/decisions", "en", "# Inherited dependencies"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			context := assetContext()
			context.Decisions, context.Language = tc.decisions, tc.language
			context.DirectRequires = []assets.Requirement{
				{Path: "example.test/library", Version: "v1.2.3"},
				{Path: "github.com/sirupsen/logrus", Version: "v1.9.3"},
				{Path: "github.com/swaggo/swag", Version: "v1.16.4"},
				{Path: "example.test/logrus", Version: "v1.0.0"},
			}
			path := tc.decisions + "/0001-inherited-baseline.md"
			got := renderedAsset(t, path, context)
			for _, want := range []string{
				tc.title, "modules:\n",
				"| example.test/library | v1.2.3 | inherited |",
				"| github.com/sirupsen/logrus | v1.9.3 | denied |",
				"| github.com/swaggo/swag | v1.16.4 | denied |",
				"| example.test/logrus | v1.0.0 | inherited |",
			} {
				if !strings.Contains(string(got.Data), want) {
					t.Errorf("baseline lacks %q", want)
				}
			}
			if got.Template.Ownership != assets.OwnedByDeveloper {
				t.Fatal("baseline is not developer-owned")
			}
		})
	}
}

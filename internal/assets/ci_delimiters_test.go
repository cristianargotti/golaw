package assets_test

import (
	"errors"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/cristianargotti/golaw/internal/assets"
)

func TestCIExpressionDoesNotRelaxTemplateExhaustion(t *testing.T) {
	t.Parallel()
	const literal = `{{printf "%s" "${{ vars.GOLAW_ENROLLED }}"}}`
	for _, test := range []struct {
		name, source, destination, body string
		fail                            bool
	}{
		{"enrollment", ciAssetSource, ciAssetPath, "value: " + literal + "\n", false},
		{"ordinary asset", "templates/a.yml.tmpl", "a.yml", "value: " + literal + "\n", true},
		{"different destination", ciAssetSource, "a.yml", "value: " + literal + "\n", true},
		{"different source", "templates/a.yml.tmpl", ciAssetPath, "value: " + literal + "\n", true},
		{"missing input", ciAssetSource, ciAssetPath, "value: {{.Absent}}\n", true},
		{"emitted input", ciAssetSource, ciAssetPath, `value: {{print "{{.Absent}}"}}`, true},
		{"other expression", ciAssetSource, ciAssetPath, `value: {{print "${{ vars.OTHER }}"}}`, true},
		{"closing delimiter", ciAssetSource, ciAssetPath, "value: left }}\n", true},
		{"ordinary value", "templates/a.yml.tmpl", "a.yml", "value: {{.KitVersion}}\n", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			tree := fstest.MapFS{
				test.source: {Data: []byte(test.body)},
			}
			entry := assets.Template{
				Source: test.source, Destination: test.destination,
				Ownership: assets.OwnedByKit,
			}
			got, err := assets.RenderContext(tree, entry, assetContext())
			if test.fail {
				requireCIContextError(t, err, test.source, test.destination)
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if test.name == "enrollment" &&
				string(got.Data) != "value: "+ciEnrollmentExpression+"\n" {
				t.Fatalf("expression changed: %q", got.Data)
			}
		})
	}
}

func requireCIContextError(t *testing.T, err error, source, destination string) {
	t.Helper()
	if !errors.Is(err, assets.ErrRender) {
		t.Fatalf("expected contextual render error, got %v", err)
	}
	if !strings.Contains(err.Error(), source) || !strings.Contains(err.Error(), destination) {
		t.Fatalf("render error does not name both paths: %v", err)
	}
}

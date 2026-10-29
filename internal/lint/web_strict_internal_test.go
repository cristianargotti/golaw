package lint

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

const (
	webStrictManifest = "testdata/web/strict.yml"
	webFrameManifest  = "testdata/web/frame.yml"
)

func TestWebStrictManifestIsBound(t *testing.T) {
	t.Parallel()
	webManifestBound(t, webStrictManifest)
	webManifestBound(t, webFrameManifest)
}

func TestWebStrictSetIsProven(t *testing.T) {
	t.Parallel()
	proveWebSet(t, webStrictManifest, model.SurfaceStrict, []Driver{eslintDriver(), stylelintDriver()})
}

// The parity frame alone (sonarjs recommended, the HTML extraction, the kit
// configuration read in place of the repository's) before any row exists.
func TestWebParityFrameIsProven(t *testing.T) {
	t.Parallel()
	proveWebSet(t, webFrameManifest, model.SurfaceParity, []Driver{eslintDriver(), stylelintDriver()})
}

// One HTML file meets the markup rules of @html-eslint and the script rules
// through eslint-plugin-html in the same run (the manual configuration the
// eslint-plugin-html README documents).
func TestWebStrictHTMLRunsBothPlugins(t *testing.T) {
	t.Parallel()
	d := eslintDriver()
	files := webFixture(t, "testdata/web/strict/html-eslint-require-lang/red")
	path := webToolPath(t, d.Tool)
	dir := webMaterialize(t, d, model.SurfaceStrict)
	issues := webIssues(t, d, path, dir, model.SurfaceStrict, files)
	for _, code := range []string{"curly", "html-eslint-require-lang"} {
		if webCount(issues, code) == 0 {
			t.Errorf("the strict run of one HTML file yields no %s", code)
		}
	}
}

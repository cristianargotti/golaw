package assets_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

func TestTheSurfaceTargetsAreRequiredAndRendered(t *testing.T) {
	t.Parallel()
	if got := policyfiles.RequiredTargets(nil); !slices.Equal(got, []string{"scan", "lint-surface"}) {
		t.Errorf("no capability = %v, want scan lint-surface", got)
	}
	goTargets := policyfiles.RequiredTargets([]string{"go"})
	at := slices.Index(goTargets, "coverage-new")
	if at < 0 || goTargets[at-1] != "coverage-check" {
		t.Errorf("go targets = %v, want coverage-new right after coverage-check", goTargets)
	}
	context := assetContext()
	context.Capabilities = model.Capabilities()
	body := string(renderedAsset(t, "golaw.mk", context).Data)
	for _, want := range []string{
		"\nlint-surface:\n\tgolaw lint --surface .\n",
		"\ncoverage-new:\n\tgo test -coverprofile profile.cov ./...\n\tgolaw coverage --new-code --input profile.cov $(if $(COVERAGE_BASE),--base $(COVERAGE_BASE))\n",
		"\ndockerfile-lint:\n\tgolaw lint --only dockerfile .\n",
		"\nCOVERAGE_BASE ?=\n",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("golaw.mk lacks %q", want)
		}
	}
	if strings.Contains(body, "hadolint Dockerfile") {
		t.Error("dockerfile-lint still reads the root Dockerfile only")
	}
}

func TestTheCITemplateProvisionsTheSurfaceTools(t *testing.T) {
	t.Parallel()
	hadolint, ok := toolchain.Lookup("hadolint")
	if !ok {
		t.Fatal("hadolint is not pinned")
	}
	var linux toolchain.Asset
	for _, asset := range hadolint.Assets {
		if asset.Platform == "linux-amd64" {
			linux = asset
		}
	}
	for _, caps := range [][]string{nil, {"go"}, model.Capabilities()} {
		requireSurfaceCI(t, caps, linux)
	}
}

// requireSurfaceCI requires one rendering of the CI file to provision the
// surface tools in each job that runs them, at the toolchain pins.
func requireSurfaceCI(t *testing.T, caps []string, linux toolchain.Asset) {
	t.Helper()
	context := assetContext()
	context.Capabilities = caps
	body := string(renderedAsset(t, ".github/workflows/ci.yml", context).Data)
	for _, want := range []string{
		"\n  surface:\n",
		"actions/setup-python@5fda3b95a4ea91299a34e894583c3862153e4b97 # v7.0.0",
		"actions/cache@55cc8345863c7cc4c66a329aec7e433d2d1c52a9 # v6.1.0",
		"key: golaw-tools-${{ runner.os }}-v" + context.KitVersion,
		"- run: golaw setup --surface\n",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("%v: the CI file lacks %q", caps, want)
		}
	}
	if slices.Contains(caps, "image") && (!strings.Contains(body, linux.URL) || !strings.Contains(body, linux.SHA256)) {
		t.Errorf("the image download is not the toolchain pin %s", linux.URL)
	}
	wantSurface := "- run: make lint-surface\n"
	if slices.Contains(caps, "go") {
		wantSurface = "- run: make lint-surface coverage-new\n"
	}
	if !strings.Contains(body, wantSurface) {
		t.Errorf("%v: the surface job does not run %q", caps, wantSurface)
	}
	// golaw lint --only dockerfile runs every driver of the language,
	// checkov's dockerfile framework under strict among them.
	if image := ciJob(body, "image"); slices.Contains(caps, "image") && !strings.Contains(image, "- run: golaw setup --surface\n") {
		t.Errorf("%v: the image job runs dockerfile-lint without the surface tools:\n%s", caps, image)
	}
}

// ciJob answers the lines of one job of the rendered CI file, up to the next
// job; empty when the job is absent.
func ciJob(body, name string) string {
	_, rest, found := strings.Cut(body, "\n  "+name+":\n")
	if !found {
		return ""
	}
	var job strings.Builder
	for _, line := range strings.SplitAfter(rest, "\n") {
		if len(line) > 2 && strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "   ") {
			break
		}
		_, _ = job.WriteString(line)
	}
	return job.String()
}

package lint

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

// The proof manifests of the infrastructure assets, relative to the package.
const (
	infraManifest        = "proof/infra.yml"
	strictToolsManifest  = "testdata/infra/strict-tools.yml"
	strictDockerManifest = "testdata/infra/strict-docker.yml"
	infraKit             = "0.0.0-test"
)

// infraSets lists the two rule sets every parity row must hold under.
var infraSets = []string{model.SurfaceParity, model.SurfaceStrict}

// infraToolNames lists the tools a proof entry of this part may name.
var infraToolNames = []string{"cfn-lint", "checkov", "hadolint", "shellcheck", "yamllint"}

// infraYAML decodes one configuration file of a set as a YAML mapping.
func infraYAML(t *testing.T, set, name string) map[string]any {
	t.Helper()
	data, err := fs.ReadFile(infraConfig(), set+"/"+name)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("%s/%s: %v", set, name, err)
	}
	return doc
}

// infraStrings reads a YAML sequence of strings.
func infraStrings(t *testing.T, value any) []string {
	t.Helper()
	items, ok := value.([]any)
	if !ok {
		t.Fatalf("%v is not a sequence", value)
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			t.Fatalf("%v is not a string", item)
		}
		out = append(out, text)
	}
	return out
}

func TestInfraConfigTreeHoldsOneFilePerToolAndSet(t *testing.T) {
	t.Parallel()
	want := map[string][]string{
		model.SurfaceParity: {hadolintConfig, checkovConfig},
		model.SurfaceStrict: {cfnLintConfig, hadolintConfig, yamllintConfig, checkovConfig},
	}
	for set, names := range want {
		entries, err := fs.ReadDir(infraConfig(), set)
		if err != nil {
			t.Fatal(err)
		}
		got := make([]string, 0, len(entries))
		for _, entry := range entries {
			got = append(got, entry.Name())
		}
		if !slices.Equal(got, names) {
			t.Errorf("%s holds %v, want %v", set, got, names)
		}
	}
}

// TestHadolintSetsKeepTheirThresholds pins the parity file to the default
// rules at warning and above: hadolint's JSON lists every level whatever the
// threshold says, so the info and style rules of the hadolint 2.15.1 README
// table are ignored by name, and DL3015 rises to warning because it carries
// docker:S6500.
func TestHadolintSetsKeepTheirThresholds(t *testing.T) {
	t.Parallel()
	parity := infraYAML(t, model.SurfaceParity, hadolintConfig)
	if parity["failure-threshold"] != "warning" {
		t.Errorf("parity threshold = %v, want warning", parity["failure-threshold"])
	}
	below := []string{"DL3001", "DL3009", "DL3010", "DL3019", "DL3047", "DL3048", "DL3049", "DL3050", "DL3059", "DL3060", "DL3066"}
	if got := infraStrings(t, parity["ignored"]); !slices.Equal(got, below) {
		t.Errorf("parity ignores %v, want %v", got, below)
	}
	override, ok := parity["override"].(map[string]any)
	if !ok || !slices.Equal(infraStrings(t, override["warning"]), []string{"DL3015"}) {
		t.Errorf("parity override = %v, want warning: [DL3015]", parity["override"])
	}
	strict := infraYAML(t, model.SurfaceStrict, hadolintConfig)
	if strict["failure-threshold"] != "style" || strict["ignored"] != nil || strict["override"] != nil {
		t.Errorf("strict = %v, want only failure-threshold: style", strict)
	}
}

func TestCheckovSetsNameTheirFrameworksAndStayOffline(t *testing.T) {
	t.Parallel()
	parity := infraYAML(t, model.SurfaceParity, checkovConfig)
	if got := infraStrings(t, parity["framework"]); !slices.Equal(got, []string{"cloudformation"}) {
		t.Errorf("parity frameworks = %v", got)
	}
	checks := []string{"CKV_AWS_18", "CKV_AWS_20", "CKV_AWS_21", "CKV_AWS_53", "CKV_AWS_54", "CKV_AWS_55", "CKV_AWS_56", "CKV_AWS_57", "CKV_AWS_91"}
	if got := infraStrings(t, parity["check"]); !slices.Equal(got, checks) {
		t.Errorf("parity checks = %v, want %v", got, checks)
	}
	strict := infraYAML(t, model.SurfaceStrict, checkovConfig)
	if got := infraStrings(t, strict["framework"]); !slices.Equal(got, []string{"cloudformation", "dockerfile"}) {
		t.Errorf("strict frameworks = %v", got)
	}
	if strict["check"] != nil {
		t.Errorf("strict narrows checkov to %v, want the complete set", strict["check"])
	}
	for set, doc := range map[string]map[string]any{model.SurfaceParity: parity, model.SurfaceStrict: strict} {
		if doc["skip-download"] != true {
			t.Errorf("%s checkov may download: %v", set, doc["skip-download"])
		}
	}
}

func TestYamllintAndCfnLintStrictSets(t *testing.T) {
	t.Parallel()
	doc := infraYAML(t, model.SurfaceStrict, yamllintConfig)
	if doc["extends"] != "default" {
		t.Errorf("yamllint extends %v, want default", doc["extends"])
	}
	rules, ok := doc["rules"].(map[string]any)
	if !ok {
		t.Fatalf("yamllint rules = %v, want a mapping", doc["rules"])
	}
	length, ok := rules["line-length"].(map[string]any)
	if !ok {
		t.Fatalf("line-length = %v, want a mapping", rules["line-length"])
	}
	if ignore, isText := length["ignore"].(string); !isText || strings.TrimSpace(ignore) != "**/api/**" {
		t.Errorf("line-length ignores %q, want the OpenAPI documents under api/", ignore)
	}
	truthy, ok := rules["truthy"].(map[string]any)
	if !ok || truthy["check-keys"] != false {
		t.Errorf("truthy = %v, want check-keys: false", truthy)
	}
	cfn := infraYAML(t, model.SurfaceStrict, cfnLintConfig)
	if checks, ok := cfn["ignore_checks"].([]any); !ok || len(checks) != 0 {
		t.Errorf("cfn-lint ignore_checks = %v, want an empty list", cfn["ignore_checks"])
	}
}

func TestInfraConfigMaterializesIntoTheToolDirectory(t *testing.T) {
	t.Parallel()
	tools := t.TempDir()
	for _, set := range infraSets {
		sub, err := fs.Sub(infraConfig(), set)
		if err != nil {
			t.Fatal(err)
		}
		dir, err := toolchain.Materialize(tools, infraKit, set, infraConfig())
		if err != nil {
			t.Fatal(err)
		}
		if want := toolchain.ConfigDir(tools, infraKit, set); dir != want {
			t.Errorf("%s materialized into %s, want %s", set, dir, want)
		}
		want, err := fs.ReadFile(sub, hadolintConfig)
		if err != nil {
			t.Fatal(err)
		}
		if got, err := os.ReadFile(filepath.Join(dir, hadolintConfig)); err != nil || !bytes.Equal(got, want) {
			t.Errorf("%s %s = %q, %v", set, hadolintConfig, got, err)
		}
	}
}

// infraManifestEntries loads a manifest, proves every entry is
// lint.<tool>-<code> of an infrastructure tool with two loadable trees, and
// answers the number of entries.
func infraManifestEntries(t *testing.T, manifest string) int {
	t.Helper()
	m, err := proof.LoadManifest(manifest)
	if err != nil {
		t.Fatalf("%s: %v", manifest, err)
	}
	for _, e := range m.Checks {
		named := func(tool string) bool { return e.ID == "lint."+tool+"-"+e.Code }
		if !slices.ContainsFunc(infraToolNames, named) {
			t.Errorf("%s: %s is not lint.<tool>-<code> of an infrastructure tool", manifest, e.ID)
		}
		for _, dir := range []string{e.Red, e.Near} {
			if _, err := proof.LoadFixture(filepath.Join(filepath.Dir(manifest), dir)); err != nil {
				t.Errorf("%s: %s: %v", manifest, e.ID, err)
			}
		}
	}
	return len(m.Checks)
}

func TestInfraProofManifestsBindLoadableFixtures(t *testing.T) {
	t.Parallel()
	want := map[string]int{infraManifest: 11, strictToolsManifest: 5, strictDockerManifest: 2}
	for manifest, count := range want {
		if got := infraManifestEntries(t, manifest); got != count {
			t.Errorf("%s holds %d entries, want %d", manifest, got, count)
		}
	}
	for _, dir := range []string{"testdata/infra/shfmt/red", "testdata/infra/shfmt/near"} {
		if _, err := proof.LoadFixture(dir); err != nil {
			t.Errorf("%s: %v", dir, err)
		}
	}
}

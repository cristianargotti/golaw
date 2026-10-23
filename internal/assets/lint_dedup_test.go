package assets_test

import (
	"slices"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestStrictAssetReportsEachDefectOnce pins one reporter per defect the
// survey of a template birth counted twice, and every finding shown in the
// first run instead of behind another on its line.
func TestStrictAssetReportsEachDefectOnce(t *testing.T) {
	t.Parallel()
	var config map[string]any
	if err := yaml.Unmarshal(renderedAsset(t, ".golangci.yml", assetContext()).Data, &config); err != nil {
		t.Fatal(err)
	}
	settings := yamlMap(t, yamlMap(t, config, "linters"), "settings")
	for _, tc := range []struct {
		name, linter, key string
		want              []string
	}{
		{"staticcheck leaves package comments and names to revive", "staticcheck", "checks", []string{"all", "-ST1000", "-ST1003"}},
		{"gosec leaves unchecked errors to errcheck", "gosec", "excludes", []string{"G104"}},
		{"gocritic leaves defers in loops to revive", "gocritic", "disabled-checks", []string{"hugeParam", "unnamedResult", "deferInLoop"}},
	} {
		values := stringList(t, tc.name, anyList(t, yamlMap(t, settings, tc.linter)[tc.key]))
		if !slices.Equal(values, tc.want) {
			t.Fatalf("%s: %s.%s = %v, want %v", tc.name, tc.linter, tc.key, values, tc.want)
		}
	}
	if yamlMap(t, config, "issues")["uniq-by-line"] != false {
		t.Fatal("issues.uniq-by-line must be false, so no finding hides behind another on its line")
	}
	enabled := anyList(t, yamlMap(t, config, "linters")["enable"])
	for _, reporter := range []any{"revive", "gosec", "gocritic"} {
		if !slices.Contains(enabled, reporter) {
			t.Fatalf("%s left the strict set", reporter)
		}
	}
	rules := anyList(t, yamlMap(t, settings, "revive")["rules"])
	for _, name := range []string{"package-comments", "var-naming", "defer"} {
		if !slices.ContainsFunc(rules, func(rule any) bool {
			fields, ok := rule.(map[string]any)
			return ok && fields["name"] == name
		}) {
			t.Fatalf("revive lost %s, the one reporter it keeps", name)
		}
	}
}

// stringList is a YAML sequence of strings; a failure names the setting it
// came from.
func stringList(t *testing.T, name string, list []any) []string {
	t.Helper()
	values := make([]string, 0, len(list))
	for _, value := range list {
		text, ok := value.(string)
		if !ok {
			t.Fatalf("%s: %v is not a string", name, value)
		}
		values = append(values, text)
	}
	return values
}

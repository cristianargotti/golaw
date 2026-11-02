package assets_test

import (
	"reflect"
	"slices"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestLintAssetLetsGeneratedCodeCallTheSupportPackage pins the two lines
// every generated use case and hole needs in the adopter's own make lint.
func TestLintAssetLetsGeneratedCodeCallTheSupportPackage(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, support, want string
		hex                 bool
	}{
		{"red: the default support package in a hexagonal profile", "", "internal/gensupport", true},
		{"red: a support package the policy moves", "pkg/gen/support", "pkg/gen/support", true},
		{"near: an ordinary profile has no application rule", "", "internal/gensupport", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			settings := supportLintSettings(t, tc.support, tc.hex)
			support := assetContext().ModulePath + "/" + tc.want
			globs := yamlMap(t, settings, "wrapcheck")["ignore-package-globs"]
			if !reflect.DeepEqual(globs, []any{support, support + "/*"}) {
				t.Errorf("wrapcheck ignore-package-globs = %v, want the support package %s and its packages", globs, support)
			}
			rules := yamlMap(t, settings, "depguard")
			application, ok := yamlMap(t, rules, "rules")["application"].(map[string]any)
			if ok != tc.hex {
				t.Fatalf("application rule present = %v, want %v", ok, tc.hex)
			}
			if tc.hex && !slices.Contains(anyList(t, application["allow"]), any(support)) {
				t.Errorf("application allow = %v, want %s beside the domain and the application", application["allow"], support)
			}
		})
	}
}

// supportLintSettings renders the lint asset of a go profile, hexagonal
// when asked, with the support package of the policy, and answers its
// linters.settings.
func supportLintSettings(t *testing.T, support string, hex bool) map[string]any {
	t.Helper()
	context := assetContext()
	context.Capabilities = []string{"go"}
	if hex {
		context.Capabilities = append(context.Capabilities, "hexagonal")
	}
	context.Generate.Support = support
	var config map[string]any
	if err := yaml.Unmarshal(renderedAsset(t, ".golangci.yml", context).Data, &config); err != nil {
		t.Fatal(err)
	}
	return yamlMap(t, yamlMap(t, config, "linters"), "settings")
}

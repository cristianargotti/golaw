package assets_test

import (
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/assets"
)

// strictAdditions are the linters of inventory 4.5 the kit, a command line
// tool, leaves out and every adopter gets.
var strictAdditions = []any{"promlinter", "rowserrcheck", "spancheck", "sqlclosecheck", "tagliatelle", "testifylint", "unqueryvet"}

func TestStrictLinterRetainsEveryUnchangedSetting(t *testing.T) {
	t.Parallel()
	ordinary := assetContext()
	ordinary.Capabilities = []string{"go"}
	file := renderedAsset(t, ".golangci.yml", ordinary)
	assertAsset(t, file.Path, string(file.Data), assets.OwnedByKit)
	for _, tc := range []struct {
		name string
		caps []string
		hex  bool
	}{
		{"ordinary", []string{"go"}, false},
		{"hexagonal", []string{"go", "hexagonal"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			context := assetContext()
			context.Capabilities = tc.caps
			got := renderedAsset(t, ".golangci.yml", context)
			if got.Template.Ownership != assets.OwnedByKit {
				t.Fatal("linter ownership is not kit")
			}
			var actual map[string]any
			if err := yaml.Unmarshal(got.Data, &actual); err != nil {
				t.Fatal(err)
			}
			sortLinters(t, actual)
			settings := yamlMap(t, yamlMap(t, actual, "linters"), "settings")
			if rules := yamlMap(t, settings, "depguard")["rules"]; !reflect.DeepEqual(rules, wantDepguard(context.ModulePath, tc.hex)) {
				t.Fatalf("depguard = %+v", rules)
			}
			want := kitLintConfig(t)
			delete(settings, "depguard")
			delete(yamlMap(t, yamlMap(t, want, "linters"), "settings"), "depguard")
			if !reflect.DeepEqual(want, actual) {
				t.Fatalf("a setting outside the declared substitutions changed:\n%s", got.Data)
			}
		})
	}
}

// kitLintConfig is the kit's .golangci.yml with the substitutions of the
// asset: the module path, the seven linters of inventory 4.5 with the
// settings it names, forbidigo keeping float32 alone, because the wall
// clock is goast.wall-clock's in domain and application (section 6), and
// neither the iface exclusion of the kit's model nor the clock exclusion of
// its cmd.
func kitLintConfig(t *testing.T) map[string]any {
	t.Helper()
	data, err := os.ReadFile("../../.golangci.yml")
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := yaml.Unmarshal([]byte(strings.ReplaceAll(string(data), "github.com/cristianargotti/golaw", templateValues().ModulePath)), &config); err != nil {
		t.Fatal(err)
	}
	linters := yamlMap(t, config, "linters")
	linters["enable"] = append(anyList(t, linters["enable"]), strictAdditions...)
	sortLinters(t, config)
	settings := yamlMap(t, linters, "settings")
	forbidigo := yamlMap(t, settings, "forbidigo")
	forbidigo["forbid"] = anyList(t, forbidigo["forbid"])[:1]
	settings["tagliatelle"] = map[string]any{"case": map[string]any{"rules": map[string]any{"json": "camel"}}}
	settings["testifylint"] = map[string]any{"enable-all": true}
	exclusions := yamlMap(t, linters, "exclusions")
	exclusions["rules"] = slices.DeleteFunc(anyList(t, exclusions["rules"]), func(rule any) bool {
		fields, ok := rule.(map[string]any)
		if !ok {
			t.Fatal("exclusion rule is not a mapping")
		}
		return fields["path"] == "internal/model/" || fields["path"] == `(^cmd/|_test\.go)`
	})
	return config
}

// wantDepguard is the depguard mapping the asset renders: the lax main
// rule of every adopter and the layer rules of a hexagonal one.
func wantDepguard(module string, hex bool) map[string]any {
	rules := map[string]any{"main": map[string]any{"list-mode": "lax", "deny": []any{
		map[string]any{"pkg": "io/ioutil", "desc": "deprecated since Go 1.16, use io and os"},
		map[string]any{"pkg": "github.com/pkg/errors", "desc": "use fmt.Errorf with %w and the standard errors package"},
		map[string]any{"pkg": "math/rand$", "desc": "use math/rand/v2 or crypto/rand"},
		map[string]any{"pkg": "github.com/sirupsen/logrus", "desc": "use log/slog"},
	}}}
	if hex {
		rules["domain"] = map[string]any{
			"list-mode": "strict", "files": []any{"!$test", "**/internal/domain/**"},
			"allow": []any{"$gostd", module + "/internal/domain"},
		}
		rules["application"] = map[string]any{
			"list-mode": "strict", "files": []any{"!$test", "**/internal/application/**"},
			"allow": []any{"$gostd", module + "/internal/domain", module + "/internal/application"},
		}
	}
	return rules
}

// sortLinters orders the enabled linters of a configuration, so two lists
// compare as the sets golangci-lint reads.
func sortLinters(t *testing.T, config map[string]any) {
	t.Helper()
	linters := yamlMap(t, config, "linters")
	enable := anyList(t, linters["enable"])
	slices.SortFunc(enable, func(a, b any) int { return strings.Compare(fmt.Sprint(a), fmt.Sprint(b)) })
	linters["enable"] = enable
}

// anyList is a YAML sequence.
func anyList(t *testing.T, value any) []any {
	t.Helper()
	list, ok := value.([]any)
	if !ok {
		t.Fatalf("%v is not a sequence", value)
	}
	return list
}

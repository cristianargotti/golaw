package policyfiles_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
)

// layerModule is the go.mod of the hexagonal fixtures.
const layerModule = "module example.test/service\n\ngo 1.27\n"

// layerRules renders the depguard rules of a hexagonal configuration: the
// list mode and the allow lists of the domain and the application rules.
func layerRules(domainMode string, domainAllow, applicationAllow []string) string {
	return "    depguard:\n      rules:\n" +
		"        domain:\n          list-mode: " + domainMode + "\n" +
		"          files: [\"!$test\", \"**/internal/domain/**\"]\n" +
		"          allow: [" + strings.Join(domainAllow, ", ") + "]\n" +
		"        application:\n          list-mode: strict\n" +
		"          files: [\"!$test\", \"**/internal/application/**\"]\n" +
		"          allow: [" + strings.Join(applicationAllow, ", ") + "]\n"
}

// globalRule renders one strict depguard rule that allows the standard
// library alone, its files selector line as written or left out.
func globalRule(files string) string {
	return "    depguard:\n      rules:\n        all:\n          list-mode: strict\n" + files + "          allow: [$gostd]\n"
}

// layerRow is one hexagonal tree and the codes policy.golangci answers.
type layerRow struct {
	name   string
	rules  string // appended to the settings of allLinters
	module string // the go.mod, none when empty
	caps   []string
	want   []string
}

func TestGolangciHoldsTheLayersOfAHexagonalRepository(t *testing.T) {
	t.Parallel()
	domain := []string{"$gostd", "example.test/service/internal/domain"}
	application := []string{"$gostd", "example.test/service/internal/domain", "example.test/service/internal/application"}
	hex := []string{"go", "hexagonal"}
	for _, row := range []layerRow{
		{name: "the rules init renders", rules: layerRules("strict", domain, application), module: layerModule, caps: hex},
		{name: "no layer rule", module: layerModule, caps: hex, want: []string{"golangci-layer-missing", "golangci-layer-missing"}},
		{name: "near: no layer rule without hexagonal", module: layerModule, caps: []string{"go"}},
		{name: "a lax domain rule", rules: layerRules("lax", domain, application), module: layerModule, caps: hex, want: []string{"golangci-layer-open"}},
		{name: "a domain that imports an adapter", rules: layerRules("strict", append(slices.Clone(domain), "example.test/service/internal/adapters"), application), module: layerModule, caps: hex, want: []string{"golangci-layer-open"}},
		{name: "a domain that imports the application", rules: layerRules("strict", append(slices.Clone(domain), "example.test/service/internal/application"), application), module: layerModule, caps: hex, want: []string{"golangci-layer-open"}},
		{name: "an application that imports everything", rules: layerRules("strict", domain, []string{"$all"}), module: layerModule, caps: hex, want: []string{"golangci-layer-open"}},
		{name: "near: a domain subpackage and a third-party package", rules: layerRules("strict", append(slices.Clone(domain), "example.test/service/internal/domain/money", "github.com/google/uuid"), application), module: layerModule, caps: hex},
		{name: "near: an adapter allowed without go.mod to name the module", rules: layerRules("strict", append(slices.Clone(domain), "example.test/service/internal/adapters"), application), caps: hex},
		{name: "near: one global strict rule", rules: globalRule("          files: [\"$all\", \"!$test\"]\n"), module: layerModule, caps: hex},
		{name: "near: a global strict rule without files", rules: globalRule(""), module: layerModule, caps: hex},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			files := map[string]string{".golangci.yml": allLinters + row.rules}
			if row.module != "" {
				files["go.mod"] = row.module
			}
			res := measure(t, policyfiles.Golangci(), fixture{files: files, capabilities: row.caps})
			if got := codes(res); res.Status.Kind != model.StatusMeasured || (!slices.Equal(got, row.want) && len(got)+len(row.want) != 0) {
				t.Fatalf("status %+v, codes = %v, want %v (%+v)", res.Status, got, row.want, res.Findings)
			}
		})
	}
}

func TestGolangciKeepsAStrictRuleBesideALaxOne(t *testing.T) {
	t.Parallel()
	lax := "        legacy:\n          list-mode: lax\n          files: [\"**/internal/domain/**\"]\n"
	config := allLinters + layerRules("strict", []string{"$gostd"}, []string{"$gostd"}) + lax
	res := measure(t, policyfiles.Golangci(), fixture{files: map[string]string{".golangci.yml": config, "go.mod": layerModule}, capabilities: []string{"go", "hexagonal"}})
	if len(res.Findings) != 0 {
		t.Fatalf("findings = %+v", res.Findings)
	}
	missing := measure(t, policyfiles.Golangci(), fixture{files: map[string]string{".golangci.yml": allLinters, "go.mod": layerModule}, capabilities: []string{"go", "hexagonal"}})
	domainMissing := "depguard has no strict rule over internal/domain; add the domain rule golaw init renders under linters.settings.depguard.rules of .golangci.yml"
	if len(missing.Findings) != 2 || !slices.ContainsFunc(missing.Findings, func(f model.Finding) bool { return f.Message == domainMissing && f.Line == 2 }) {
		t.Fatalf("missing = %+v", missing.Findings)
	}
}

func TestGolangciRequiresTheFloat32Pattern(t *testing.T) {
	t.Parallel()
	head := "version: \"2\"\nlinters:\n  default: all\n  settings:\n    govet:\n      enable-all: true\n"
	for _, tc := range []struct {
		name, forbid string
		want         []string
	}{
		{"no forbidigo settings", "", []string{"golangci-float32-allowed"}},
		{"near: the pattern as a mapping", "    forbidigo:\n      forbid:\n        - pattern: ^float32$\n          msg: money never travels as float32\n", nil},
		{"near: the pattern as a string", "    forbidigo:\n      forbid: [^float32$]\n", nil},
		{"near: a wider pattern", "    forbidigo:\n      forbid:\n        - pattern: ^float(32|64)$\n", nil},
		{"another type only", "    forbidigo:\n      forbid:\n        - pattern: ^float64$\n", []string{"golangci-float32-allowed"}},
		{"a pattern that does not compile", "    forbidigo:\n      forbid:\n        - pattern: (float32\n", []string{"golangci-float32-allowed"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res := measure(t, policyfiles.Golangci(), fixture{files: map[string]string{".golangci.yml": head + tc.forbid}, capabilities: []string{"go"}})
			if got := codes(res); !slices.Equal(got, tc.want) && len(got)+len(tc.want) != 0 {
				t.Fatalf("codes = %v, want %v (%+v)", got, tc.want, res.Findings)
			}
		})
	}
}

func TestGolangciNamesALayerSelectorItCannotResolve(t *testing.T) {
	t.Parallel()
	layers := "    depguard:\n      rules:\n        layers:\n          list-mode: strict\n" +
		"          files: [\"**/internal/**\"]\n          allow: [$gostd]\n"
	hex := []string{"go", "hexagonal"}
	res := measure(t, policyfiles.Golangci(), fixture{files: map[string]string{".golangci.yml": allLinters + layers, "go.mod": layerModule}, capabilities: hex})
	if !model.IsOutsideSubset(res.Status) || len(res.Findings) != 0 ||
		!strings.Contains(res.Status.Reason, ".golangci.yml:") || !strings.Contains(res.Status.Reason, "layers") {
		t.Fatalf("a selector golaw does not resolve = %+v", res)
	}
	open := strings.Replace(layers, "allow: [$gostd]", "allow: [$all]", 1)
	missing := measure(t, policyfiles.Golangci(), fixture{files: map[string]string{".golangci.yml": allLinters + open, "go.mod": layerModule}, capabilities: hex})
	if missing.Status.Kind != model.StatusMeasured || len(missing.Findings) != 2 {
		t.Fatalf("an open rule holds no layer, whatever it selects = %+v", missing)
	}
}

func TestGolangciKeepsUnsupportedLayerSettingsOutsideTheSubset(t *testing.T) {
	t.Parallel()
	head := "    depguard:\n      rules:\n        domain:\n          list-mode: strict\n"
	for _, settings := range []string{
		"    depguard: []\n",
		"    depguard:\n      rules: []\n",
		"    depguard:\n      rules:\n        domain: []\n",
		head + "          files: '**/internal/domain/**'\n",
		head + "          files: [{pattern: '**/internal/domain/**'}]\n",
		head + "          allow: $gostd\n",
		head + "          allow: [[$gostd]]\n",
		strings.Replace(head, "list-mode: strict", "list-mode: [strict]", 1),
		strings.Replace(head, "list-mode: strict", "list-mode: custom", 1),
		head + "          allow: [example.test/service]\n          deny: [{pkg: example.test/service}]\n",
	} {
		res := measure(t, policyfiles.Golangci(), fixture{
			files: map[string]string{".golangci.yml": allLinters + settings, "go.mod": layerModule}, capabilities: []string{"go", "hexagonal"},
		})
		if !model.IsOutsideSubset(res.Status) || len(res.Findings) != 0 || !strings.Contains(res.Status.Reason, ".golangci.yml:") {
			t.Errorf("settings %q = %+v", settings, res)
		}
	}
}

func TestGolangciKeepsUnsupportedFloat32SettingsOutsideTheSubset(t *testing.T) {
	t.Parallel()
	head := "version: \"2\"\nlinters:\n  default: all\n  settings:\n    govet:\n      enable-all: true\n"
	for _, settings := range []string{
		"    forbidigo: []\n",
		"    forbidigo:\n      forbid: {pattern: ^float32$}\n",
		"    forbidigo:\n      forbid: [[^float32$]]\n",
		"    forbidigo:\n      forbid: [{pattern: [^float32$]}]\n",
	} {
		res := measure(t, policyfiles.Golangci(), fixture{files: map[string]string{".golangci.yml": head + settings}, capabilities: []string{"go"}})
		if !model.IsOutsideSubset(res.Status) || len(res.Findings) != 0 || !strings.Contains(res.Status.Reason, ".golangci.yml:") {
			t.Errorf("settings %q = %+v", settings, res)
		}
	}
	settings := "    forbidigo:\n      forbid: [[^float64$], {pattern: [^float64$]}, {}, ^float32$]\n"
	judgeRow(t, golangciRow{config: head + settings})
}

func TestGolangciRecognizesEquivalentStrictLayerRules(t *testing.T) {
	t.Parallel()
	for _, rules := range []string{
		layerRules("STRICT", []string{"$gostd", "example.test/service/internal/domain$"}, []string{"$gostd"}),
		globalRule("          files: [\"!$test\"]\n"),
		globalRule("") + "        legacy: []\n",
		strings.Replace(globalRule(""), "        all:", "        legacy: []\n        all:", 1),
		strings.Replace(globalRule(""), "        all:", "        legacy:\n          files: ['**/internal/**']\n          list-mode: strict\n          allow: [$gostd]\n        all:", 1),
		globalRule("") + "          deny: [{pkg: example.test/service/internal/adapters}]\n",
		layerRules("strict", []string{"$gostd"}, []string{"$gostd"}) + "        legacy:\n          files: ['cmd/**']\n          allow: []\n",
	} {
		res := measure(t, policyfiles.Golangci(), fixture{
			files: map[string]string{".golangci.yml": allLinters + rules, "go.mod": layerModule}, capabilities: []string{"go", "hexagonal"},
		})
		if res.Status.Kind != model.StatusMeasured || len(res.Findings) != 0 {
			t.Errorf("rules %q = %+v", rules, res)
		}
	}
}

func TestGolangciNamesTheUnreadModule(t *testing.T) {
	t.Parallel()
	res := measure(t, policyfiles.Golangci(), fixture{
		files: map[string]string{".golangci.yml": allLinters, "go.mod": string(make([]byte, model.MaxReadBytes+1))}, capabilities: []string{"go", "hexagonal"},
	})
	if res.Status.Kind != model.StatusUnmeasured || len(res.Findings) != 0 || !slices.Equal(res.Unread, []string{"go.mod"}) {
		t.Fatalf("oversized module = %+v", res)
	}
}

func TestGolangciIgnoresUnsupportedAllowsOfALaxRule(t *testing.T) {
	t.Parallel()
	rules := layerRules("lax", []string{"$gostd"}, []string{"$gostd"})
	rules = strings.Replace(rules, "allow: [$gostd]", "allow: $gostd", 1)
	res := measure(t, policyfiles.Golangci(), fixture{
		files: map[string]string{".golangci.yml": allLinters + rules, "go.mod": layerModule}, capabilities: []string{"go", "hexagonal"},
	})
	if res.Status.Kind != model.StatusMeasured || !slices.Equal(codes(res), []string{"golangci-layer-open"}) {
		t.Fatalf("lax rule = %+v", res)
	}
}

func TestGolangciRestrictsOriginalRulesWithAllowLists(t *testing.T) {
	t.Parallel()
	domain := []string{"$gostd", "example.test/service/internal/domain"}
	application := []string{"$gostd", "example.test/service/internal/domain", "example.test/service/internal/application"}
	withMode := func(mode string) string {
		return strings.ReplaceAll(layerRules("strict", domain, application), "          list-mode: strict\n", mode)
	}
	denyOnly := strings.Replace(layerRules("original", nil, application), "allow: []", "deny: [{pkg: example.test/service/internal/adapters}]", 1)
	open := layerRules("original", []string{"example.test/service"}, application)
	for _, tc := range []struct {
		name, rules string
		want        []string
		limit       bool
	}{
		{name: "absent mode", rules: withMode("")},
		{name: "empty mode", rules: withMode("          list-mode: \"\"\n")},
		{name: "original mode", rules: withMode("          list-mode: original\n")},
		{name: "mixed case original mode", rules: withMode("          list-mode: OrIgInAl\n")},
		{name: "original deny only", rules: denyOnly, want: []string{"golangci-layer-open"}},
		{name: "original open allow", rules: open, want: []string{"golangci-layer-open"}},
		{name: "original deny precedence", rules: strings.Replace(open, "allow: [example.test/service]", "allow: [example.test/service]\n          deny: [{pkg: example.test/service/internal/adapters}]", 1), limit: true},
		{name: "original unsupported allow", rules: strings.Replace(open, "allow: [example.test/service]", "allow: example.test/service", 1), limit: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res := measure(t, policyfiles.Golangci(), fixture{
				files: map[string]string{".golangci.yml": allLinters + tc.rules, "go.mod": layerModule}, capabilities: []string{"go", "hexagonal"},
			})
			if tc.limit {
				if !model.IsOutsideSubset(res.Status) || len(res.Findings) != 0 || !strings.Contains(res.Status.Reason, "depguard rule domain") {
					t.Fatalf("expected domain rule limit, got %+v", res)
				}
			} else if res.Status.Kind != model.StatusMeasured || !slices.Equal(codes(res), tc.want) {
				t.Fatalf("status %v, codes = %v, want %v", res.Status, codes(res), tc.want)
			}
		})
	}
}

func TestGolangciNamesMissingLayersInsteadOfOpeningAGlobalRule(t *testing.T) {
	t.Parallel()
	main := "    depguard:\n      rules:\n        main:\n          list-mode: lax\n          deny: [{pkg: io/ioutil}]\n"
	for _, tc := range []struct{ name, rules string }{
		{"template main rule", main},
		{"global strict adapter allow", strings.Replace(globalRule(""), "[$gostd]", "[example.test/service/internal/adapters]", 1)},
		{"global all selector", strings.Replace(globalRule("          files: [$all]\n"), "strict", "lax", 1)},
		{"global negative selector", strings.Replace(globalRule("          files: ['!$test']\n"), "strict", "lax", 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res := measure(t, policyfiles.Golangci(), fixture{
				files: map[string]string{".golangci.yml": allLinters + tc.rules, "go.mod": layerModule}, capabilities: []string{"go", "hexagonal"},
			})
			want := []string{"golangci-layer-missing", "golangci-layer-missing"}
			if res.Status.Kind != model.StatusMeasured || !slices.Equal(codes(res), want) {
				t.Fatalf("status %v, codes = %v, want %v", res.Status, codes(res), want)
			}
		})
	}
}

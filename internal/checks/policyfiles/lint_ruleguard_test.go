package policyfiles_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
)

// gocriticConfig is a version 2 configuration whose gocritic runs the
// ruleguard check under the style tag with the rules the setting names.
func gocriticConfig(critic, rules string) string {
	return "version: \"2\"\nlinters:\n  settings:\n    gocritic:\n" + critic +
		"      settings:\n        ruleguard:\n          rules: '" + rules + "'\n          failOn: all\n"
}

// styleTags is the gocritic tag list of the template.
const styleTags = "      enabled-tags: [diagnostic, style, performance]\n"

// rulesFile is a ruleguard rules file whose first rule is the given match.
func rulesFile(match string) string {
	return "//go:build ruleguard\n\n// Package gorules holds the rules.\npackage gorules\n\nimport \"github.com/quasilyte/go-ruleguard/dsl\"\n\n" +
		"func stringByteCut(m dsl.Matcher) {\n\tm.Match(`" + match + "`).\n\t\tWhere(m[\"s\"].Type.Is(`string`)).\n\t\tReport(`cut at a rune start`)\n}\n"
}

// dslModule is a go.mod that requires the ruleguard dsl.
const dslModule = "module example.test/service\n\ngo 1.27\n\nrequire github.com/quasilyte/go-ruleguard/dsl v0.3.23\n"

// ruleguardTree is the tree of the template: the configuration, the rules
// file and the go.mod that requires the dsl.
func ruleguardTree() map[string]string {
	return map[string]string{
		".golangci.yml":         gocriticConfig(styleTags, "${base-path}/scripts/lint/rules.go"),
		"scripts/lint/rules.go": rulesFile("$s[:$n]"),
		"go.mod":                dslModule,
	}
}

// with answers the tree of the template with some files replaced, and a
// file removed where the value is empty.
func with(changes map[string]string) map[string]string {
	tree := ruleguardTree()
	for path, content := range changes {
		if content == "" {
			delete(tree, path)
			continue
		}
		tree[path] = content
	}
	return tree
}

func TestRuleguardJudgesTheConfigurationTheRulesAndTheModule(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		tree map[string]string
		want []string
	}{
		{"the files init renders", ruleguardTree(), nil},
		{"near: no golangci configuration, which policy.golangci names", with(map[string]string{".golangci.yml": ""}), nil},
		{"no rules setting", with(map[string]string{".golangci.yml": "version: \"2\"\nlinters:\n  settings:\n    gocritic:\n" + styleTags}), []string{"ruleguard-unconfigured"}},
		{"the ruleguard check disabled", with(map[string]string{".golangci.yml": gocriticConfig(styleTags+"      disabled-checks: [hugeParam, ruleguard]\n", "${base-path}/scripts/lint/rules.go")}), []string{"ruleguard-disabled"}},
		{"diagnostic checks alone", with(map[string]string{".golangci.yml": gocriticConfig("      enabled-tags: [diagnostic]\n", "${base-path}/scripts/lint/rules.go")}), []string{"ruleguard-disabled"}},
		{"the default checks", with(map[string]string{".golangci.yml": gocriticConfig("", "${base-path}/scripts/lint/rules.go")}), []string{"ruleguard-disabled"}},
		{"near: the check named", with(map[string]string{".golangci.yml": gocriticConfig("      enabled-checks: [ruleguard]\n", "${base-path}/scripts/lint/rules.go")}), nil},
		{"near: the experimental tag", with(map[string]string{".golangci.yml": gocriticConfig("      enabled-tags: [experimental]\n", "${base-path}/scripts/lint/rules.go")}), nil},
		{"near: every check", with(map[string]string{".golangci.yml": gocriticConfig("      enable-all: true\n", "${base-path}/scripts/lint/rules.go")}), nil},
		{"the rules file is absent", with(map[string]string{"scripts/lint/rules.go": ""}), []string{"ruleguard-rule-missing"}},
		{"the rules file holds no byte cut", with(map[string]string{"scripts/lint/rules.go": rulesFile("fmt.Sprint($x)")}), []string{"ruleguard-rule-missing"}},
		{"near: the cut written with other names", with(map[string]string{"scripts/lint/rules.go": rulesFile("$text[:$limit]")}), nil},
		{"near: the rule in the second of two files", with(map[string]string{
			".golangci.yml":         gocriticConfig(styleTags, "${config-path}/scripts/lint/other.go,${base-path}/scripts/lint/rules.go"),
			"scripts/lint/other.go": rulesFile("fmt.Sprint($x)"),
		}), nil},
		{"near: a relative path", with(map[string]string{".golangci.yml": gocriticConfig(styleTags, "scripts/lint/rules.go")}), nil},
		{"near: go.mod without the dsl, which make lint names", with(map[string]string{"go.mod": "module example.test/service\n\ngo 1.27\n"}), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res := measure(t, policyfiles.Ruleguard(), fixture{files: tc.tree, capabilities: []string{"go"}})
			if got := codes(res); res.Status.Kind != model.StatusMeasured || (!slices.Equal(got, tc.want) && len(got)+len(tc.want) != 0) {
				t.Fatalf("status %+v, codes = %v, want %v (%+v)", res.Status, got, tc.want, res.Findings)
			}
		})
	}
}

func TestRuleguardNamesWhereItCannotJudge(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		tree map[string]string
		want string
	}{
		"an unknown variable":    {with(map[string]string{".golangci.yml": gocriticConfig(styleTags, "${HOME}/rules.go")}), ".golangci.yml:8: a ruleguard rules path with a variable other than base-path or config-path"},
		"an absolute rules path": {with(map[string]string{".golangci.yml": gocriticConfig(styleTags, "/etc/rules.go")}), ".golangci.yml:8: a ruleguard rules path outside the repository"},
		"a path above the root":  {with(map[string]string{".golangci.yml": gocriticConfig(styleTags, "../rules.go")}), ".golangci.yml:8: a ruleguard rules path outside the repository"},
	} {
		res := measure(t, policyfiles.Ruleguard(), fixture{files: tc.tree, capabilities: []string{"go"}})
		if !model.IsOutsideSubset(res.Status) || !strings.HasPrefix(res.Status.Reason, tc.want+": ") {
			t.Errorf("%s = %+v, want %q", name, res.Status, tc.want)
		}
	}
	broken := measure(t, policyfiles.Ruleguard(), fixture{files: with(map[string]string{"scripts/lint/rules.go": "package gorules\nfunc {\n"}), capabilities: []string{"go"}})
	if broken.Status.Kind != model.StatusUnmeasured || model.IsOutsideSubset(broken.Status) || !strings.HasPrefix(broken.Status.Reason, "parse scripts/lint/rules.go: ") {
		t.Errorf("a rules file that does not parse = %+v", broken.Status)
	}
	off := measure(t, policyfiles.Ruleguard(), fixture{files: ruleguardTree(), capabilities: []string{"cli"}})
	if off.Status.Kind != model.StatusNotApplicable {
		t.Errorf("without go = %+v", off.Status)
	}
}

func TestRuleguardFindingsNameTheFix(t *testing.T) {
	t.Parallel()
	missing := measure(t, policyfiles.Ruleguard(), fixture{files: with(map[string]string{"scripts/lint/rules.go": ""}), capabilities: []string{"go"}})
	if len(missing.Findings) != 1 || missing.Findings[0].Path != "scripts/lint/rules.go" || missing.Findings[0].Line != 0 || missing.Findings[0].Message !=
		"the ruleguard rules file scripts/lint/rules.go that .golangci.yml names does not exist; write the stringByteCut rule golaw init renders there" {
		t.Fatalf("missing = %+v", missing.Findings)
	}
	unconfigured := measure(t, policyfiles.Ruleguard(), fixture{files: with(map[string]string{".golangci.yml": "version: \"2\"\nlinters:\n  settings:\n    gocritic:\n" + styleTags}), capabilities: []string{"go"}})
	if len(unconfigured.Findings) != 1 || unconfigured.Findings[0].Line != 4 || unconfigured.Findings[0].Message !=
		"the gocritic ruleguard setting names no rules file; set rules: '${base-path}/scripts/lint/rules.go' under linters.settings.gocritic.settings.ruleguard of .golangci.yml" {
		t.Fatalf("unconfigured = %+v", unconfigured.Findings)
	}
}

func TestRuleguardStatesUnsupportedSettingsAndRulePaths(t *testing.T) {
	t.Parallel()
	base := gocriticConfig(styleTags, "scripts/lint/rules.go")
	for _, tc := range []struct{ name, config, construct string }{
		{"scalar enabled tags", gocriticConfig("      enabled-tags: style\n", "scripts/lint/rules.go"), "enabled-tags"},
		{"scalar enabled checks", gocriticConfig("      enabled-checks: ruleguard\n", "scripts/lint/rules.go"), "enabled-checks"},
		{"scalar disabled checks", gocriticConfig(styleTags+"      disabled-checks: ruleguard\n", "scripts/lint/rules.go"), "disabled-checks"},
		{"scalar disabled tags", gocriticConfig(styleTags+"      disabled-tags: style\n", "scripts/lint/rules.go"), "disabled-tags"},
		{"mapping boolean", gocriticConfig("      enable-all: {}\n", "scripts/lint/rules.go"), "enable-all"},
		{"unknown boolean", gocriticConfig("      disable-all: yes\n", "scripts/lint/rules.go"), "disable-all"},
		{"scalar gocritic", "version: '2'\nlinters:\n  settings:\n    gocritic: style\n", "gocritic"},
		{"sequence settings", strings.Replace(base, "      settings:\n", "      settings: []\n      ignored:\n", 1), "settings"},
		{"sequence ruleguard", strings.Replace(base, "        ruleguard:\n", "        ruleguard: []\n        ignored:\n", 1), "ruleguard"},
		{"mapping rules", strings.Replace(base, "rules: 'scripts/lint/rules.go'", "rules: {}", 1), "rules"},
		{"sequence rules", strings.Replace(base, "rules: 'scripts/lint/rules.go'", "rules: [scripts/lint/rules.go]", 1), "rules"},
		{"glob", gocriticConfig(styleTags, "scripts/lint/*.go"), "glob"},
		{"glob character class", gocriticConfig(styleTags, "scripts/lint/rules[12].go"), "glob"},
		{"Windows absolute", gocriticConfig(styleTags, "C:/rules.go"), "outside the repository"},
		{"Windows separators", gocriticConfig(styleTags, `C:\rules.go`), "rules path"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res := measure(t, policyfiles.Ruleguard(), fixture{files: with(map[string]string{".golangci.yml": tc.config}), capabilities: []string{"go"}})
			if !model.IsOutsideSubset(res.Status) || len(res.Findings) != 0 || !strings.HasPrefix(res.Status.Reason, ".golangci.yml:") || !strings.Contains(res.Status.Reason, tc.construct) {
				t.Fatalf("unsupported %s = %+v", tc.name, res)
			}
		})
	}
}

func TestRuleguardFollowsExplicitTagsAfterDefaults(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, critic string
		want         []string
	}{
		{"style after disable-all", "      disable-all: true\n" + styleTags, nil},
		{"experimental after disable-all", "      disable-all: true\n      enabled-tags: [experimental]\n", nil},
		{"named after disable-all", "      disable-all: true\n      enabled-checks: [ruleguard]\n", nil},
		{"style disabled", styleTags + "      disabled-tags: [style]\n", []string{"ruleguard-disabled"}},
		{"experimental disabled", "      enabled-checks: [ruleguard]\n      disabled-tags: [experimental]\n", []string{"ruleguard-disabled"}},
		{"unrelated disabled tag", styleTags + "      disabled-tags: [performance]\n", nil},
		{"quoted booleans", "      enable-all: 'TRUE'\n      disable-all: 'false'\n", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			config := gocriticConfig(tc.critic, "scripts/lint/rules.go")
			res := measure(t, policyfiles.Ruleguard(), fixture{files: with(map[string]string{".golangci.yml": config}), capabilities: []string{"go"}})
			if res.Status.Kind != model.StatusMeasured || !slices.Equal(codes(res), tc.want) {
				t.Fatalf("status %+v, codes = %v, want %v", res.Status, codes(res), tc.want)
			}
		})
	}
}

func TestRuleguardStatesOnlyPatternLimitsItsVerdictDependsOn(t *testing.T) {
	t.Parallel()
	named := strings.Replace(rulesFile("$s[:$n]"), "m.Match(`$s[:$n]`)", "m.Match(cut)", 1) + "\nconst cut = `$s[:$n]`\n"
	joined := strings.Replace(rulesFile("$s[:$n]"), "m.Match(`$s[:$n]`)", "m.Match(`$s` + `[:$n]`)", 1)
	for _, tc := range []struct {
		name  string
		tree  map[string]string
		limit bool
	}{
		{"named constant", with(map[string]string{"scripts/lint/rules.go": named}), true},
		{"concatenated constant", with(map[string]string{"scripts/lint/rules.go": joined}), true},
		{"literal in the same file", with(map[string]string{"scripts/lint/rules.go": named + "\nfunc literal(m dsl.Matcher) { m.Match(`$s[:$n]`) }\n"}), false},
		{"literal in a later file", with(map[string]string{
			".golangci.yml":         gocriticConfig(styleTags, "scripts/lint/other.go,scripts/lint/rules.go"),
			"scripts/lint/other.go": named,
		}), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res := measure(t, policyfiles.Ruleguard(), fixture{files: tc.tree, capabilities: []string{"go"}})
			if tc.limit {
				if !model.IsOutsideSubset(res.Status) || len(res.Findings) != 0 || !strings.HasPrefix(res.Status.Reason, "scripts/lint/rules.go:9: a ruleguard Match pattern that is not a string literal: ") {
					t.Fatalf("constant pattern = %+v", res)
				}
			} else if res.Status.Kind != model.StatusMeasured || len(res.Findings) != 0 {
				t.Fatalf("known byte cut = %+v", res)
			}
		})
	}
}

package policyfiles_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
)

// allLinters is a version 2 configuration that runs every linter with the
// shadow analyzer of govet and the float32 pattern of forbidigo, the clean
// base each row below changes.
const allLinters = "version: \"2\"\nlinters:\n  default: all\n  settings:\n    forbidigo:\n      forbid:\n        - pattern: ^float32$\n    govet:\n      enable-all: true\n"

// golangciRow is one tree, the policy values it is judged under and the
// codes policy.golangci must answer.
type golangciRow struct {
	name    string
	config  string // the .golangci.yml, none when empty
	allowed []string
	want    []string
}

func TestGolangciJudgesWhichLintersRun(t *testing.T) {
	t.Parallel()
	for _, row := range []golangciRow{
		{name: "every linter with shadow", config: allLinters},
		{name: "a required linter disabled", config: allLinters + "  disable: [errcheck, wsl_v5]\n", want: []string{"golangci-linter-disabled"}},
		{name: "near: linters outside the set disabled", config: allLinters + "  disable: [exhaustruct, wsl_v5]\n"},
		{name: "two required linters disabled", config: allLinters + "  disable:\n    - funlen\n    - gosec\n", want: []string{"golangci-linter-disabled", "golangci-linter-disabled"}},
		{name: "the standard set alone", config: strings.Replace(allLinters, "  default: all\n", "", 1), want: []string{"golangci-linter-off"}},
		{name: "nothing enabled", config: strings.Replace(allLinters, "default: all", "default: none", 1), want: []string{"golangci-linter-off"}},
		{name: "no configuration", want: []string{"golangci-missing"}},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			judgeRow(t, row)
		})
	}
}

func TestGolangciRefusesAnExclusionThatHollowsARequiredLinter(t *testing.T) {
	t.Parallel()
	rules := func(text string) string { return allLinters + "  exclusions:\n    rules:\n" + text }
	for _, row := range []golangciRow{
		{name: "a test path the policy allows", config: rules("      - path: _test\\.go\n        linters: [funlen, mnd]\n"), allowed: []string{`_test\.go:funlen`, `_test\.go:mnd`}},
		{name: "a test path the policy does not allow", config: rules("      - path: _test\\.go\n        linters: [funlen, mnd]\n"), allowed: []string{`_test\.go:funlen`}, want: []string{"golangci-exclusion-unlisted"}},
		{name: "a rule without a path", config: rules("      - linters: [errcheck]\n"), want: []string{"golangci-hollowed"}},
		{name: "a rule that frees everything outside a path", config: rules("      - path-except: ^internal/\n        linters: [gosec]\n"), want: []string{"golangci-hollowed"}},
		{name: "near: a rule narrowed by its text", config: rules("      - linters: [revive]\n        text: function-length\n")},
		{name: "near: a rule narrowed by its source", config: rules("      - source: \"^//go:generate \"\n        linters: [lll]\n")},
		{name: "near: a rule of a linter outside the set", config: rules("      - linters: [wsl_v5]\n")},
		{name: "a rule of every linter under a path", config: rules("      - path: ^gen/\n"), want: []string{"golangci-exclusion-unlisted"}},
		{name: "an excluded path", config: allLinters + "  exclusions:\n    paths: [third_party/]\n", want: []string{"golangci-exclusion-unlisted"}},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			judgeRow(t, row)
		})
	}
}

func TestGolangciRequiresTheShadowAnalyzer(t *testing.T) {
	t.Parallel()
	head := "version: \"2\"\nlinters:\n  default: all\n  settings:\n    forbidigo:\n      forbid: [^float32$]\n"
	for _, row := range []golangciRow{
		{name: "govet without settings", config: head, want: []string{"golangci-shadow-off"}},
		{name: "near: shadow named alone", config: head + "    govet:\n      enable: [shadow]\n"},
		{name: "shadow disabled under enable-all", config: head + "    govet:\n      enable-all: true\n      disable: [fieldalignment, shadow]\n", want: []string{"golangci-shadow-off"}},
		{name: "near: another analyzer disabled", config: head + "    govet:\n      enable-all: true\n      disable: [fieldalignment]\n"},
		{name: "shadow named under disable-all", config: head + "    govet:\n      disable-all: true\n      enable: [shadow]\n"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			judgeRow(t, row)
		})
	}
}

func TestGolangciNamesWhereItCannotJudge(t *testing.T) {
	t.Parallel()
	fast := measure(t, policyfiles.Golangci(), fixture{
		files:        map[string]string{".golangci.yml": "version: \"2\"\nlinters:\n  default: fast\n"},
		capabilities: []string{"go"},
	})
	if !model.IsOutsideSubset(fast.Status) || !strings.HasPrefix(fast.Status.Reason, ".golangci.yml:3: linters.default fast leaves ") {
		t.Errorf("default fast without the required linters = %+v", fast.Status)
	}
	v1 := measure(t, policyfiles.Golangci(), fixture{files: map[string]string{".golangci.yml": "linters:\n  enable: [gosec]\n"}, capabilities: []string{"go"}})
	if !model.IsOutsideSubset(v1.Status) || len(v1.Findings) != 0 {
		t.Errorf("a version 1 configuration = %+v", v1)
	}
	big := measure(t, policyfiles.Golangci(), fixture{files: map[string]string{".golangci.yml": strings.Repeat("#", model.MaxReadBytes+1)}, capabilities: []string{"go"}})
	if big.Status.Kind != model.StatusUnmeasured || !slices.Equal(big.Unread, []string{".golangci.yml"}) {
		t.Errorf("a configuration over the read bound = %+v", big)
	}
	off := measure(t, policyfiles.Golangci(), fixture{capabilities: []string{"cli"}})
	if off.Status.Kind != model.StatusNotApplicable || off.Status.Reason != "capability go is not enabled" {
		t.Errorf("without go = %+v", off.Status)
	}
}

func TestGolangciFindingsNameTheLineAndTheFix(t *testing.T) {
	t.Parallel()
	res := measure(t, policyfiles.Golangci(), fixture{
		files:        map[string]string{".golangci.yml": allLinters + "  disable:\n    - wsl_v5\n    - errcheck\n"},
		capabilities: []string{"go"},
		mode:         model.ModeStaged,
	})
	if len(res.Findings) != 1 {
		t.Fatalf("findings = %+v", res.Findings)
	}
	got := res.Findings[0]
	if got.Path != ".golangci.yml" || got.Line != 12 || got.Check != "policy.golangci" || got.Rule != "rules/4.5@1" ||
		got.Message != "linter errcheck is required and listed in linters.disable; remove it from linters.disable of .golangci.yml" {
		t.Fatalf("finding = %+v", got)
	}
	missing := measure(t, policyfiles.Golangci(), fixture{capabilities: []string{"go"}})
	if len(missing.Findings) != 1 || missing.Findings[0].Path != ".golangci.yml" || missing.Findings[0].Line != 0 {
		t.Fatalf("missing = %+v", missing.Findings)
	}
}

// judgeRow measures policy.golangci over the row's tree under the go
// capability and the row's allowed exclusions.
func judgeRow(t *testing.T, row golangciRow) {
	t.Helper()
	files := map[string]string{}
	if row.config != "" {
		files[".golangci.yml"] = row.config
	}
	f := fixture{files: files, capabilities: []string{"go"}}
	if row.allowed != nil {
		f.options = model.Options{"allowed_exclusions": model.StringsValue(row.allowed...)}
	}
	res := measure(t, policyfiles.Golangci(), f)
	if res.Status.Kind != model.StatusMeasured {
		t.Fatalf("status = %+v", res.Status)
	}
	if got := codes(res); !slices.Equal(got, row.want) && (len(got) != 0 || len(row.want) != 0) {
		t.Fatalf("codes = %v, want %v (%+v)", got, row.want, res.Findings)
	}
}

func TestGolangciLeavesUnsupportedShadowSettingsUnmeasured(t *testing.T) {
	t.Parallel()
	head := "version: \"2\"\nlinters:\n  default: all\n  settings:\n"
	for _, tc := range []struct{ settings, reason string }{
		{"    govet: shadow\n", ".golangci.yml:5: linters.settings.govet that is not a mapping"},
		{"    govet: [shadow]\n", ".golangci.yml:5: linters.settings.govet that is not a mapping"},
		{"    govet:\n      enable: shadow\n", ".golangci.yml:6: linters.settings.govet.enable that is not a list of names"},
		{"    govet:\n      disable: {shadow: true}\n", ".golangci.yml:6: linters.settings.govet.disable that is not a list of names"},
		{"    govet:\n      enable: [{name: shadow}]\n", ".golangci.yml:6: linters.settings.govet.enable that is not a list of names"},
		{"    govet:\n      enable-all: [true]\n", ".golangci.yml:6: linters.settings.govet.enable-all that is not a boolean"},
		{"    govet:\n      enable-all: tRuE\n", ".golangci.yml:6: linters.settings.govet.enable-all that is not a boolean"},
		{"    govet:\n      enable-all: true\n      disable-all: fAlSe\n", ".golangci.yml:7: linters.settings.govet.disable-all that is not a boolean"},
		{"    govet:\n      enable-all: true\n      disable-all: auto\n", ".golangci.yml:7: linters.settings.govet.disable-all that is not a boolean"},
	} {
		res := measure(t, policyfiles.Golangci(), fixture{
			files: map[string]string{".golangci.yml": head + tc.settings}, capabilities: []string{"go"},
		})
		if !model.IsOutsideSubset(res.Status) || len(res.Findings) != 0 || !strings.HasPrefix(res.Status.Reason, tc.reason+": ") {
			t.Errorf("%s = %+v, want stated limit %q", tc.settings, res, tc.reason)
		}
	}
}

func TestGolangciReadsYAMLBooleanCaseForShadow(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"True", "TRUE"} {
		config := strings.Replace(allLinters, "enable-all: true", "enable-all: "+value+"\n      disable-all: FALSE", 1)
		judgeRow(t, golangciRow{config: config})
	}
}

func TestGolangciLeavesPathExceptionsOutsideTheSubset(t *testing.T) {
	t.Parallel()
	res := measure(t, policyfiles.Golangci(), fixture{
		files:        map[string]string{".golangci.yml": allLinters + "  exclusions:\n    paths-except: [^internal/]\n"},
		capabilities: []string{"go"},
	})
	if !model.IsOutsideSubset(res.Status) || len(res.Findings) != 0 ||
		!strings.HasPrefix(res.Status.Reason, ".golangci.yml:11: linters.exclusions.paths-except: ") {
		t.Fatalf("path exceptions = %+v", res)
	}
	judgeRow(t, golangciRow{config: allLinters + "  exclusions:\n    paths-except: []\n"})
}

package policyfiles

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// lintConfig is a version 2 configuration that uses every part of the
// subset: a merge key under linters, a flow and a block list, exclusion
// rules with and without a linter list, an excluded path and settings.
const lintConfig = `version: "2"
base: &base
  default: none
linters:
  <<: *base
  enable: [errcheck, govet]
  disable:
    - wsl
  exclusions:
    generated: lax
    rules:
      - path: _test\.go
        linters: [funlen, mnd]
      - path-except: ^internal/
        text: shadows
        source: ^// keep
    paths:
      - third_party/
  settings:
    govet:
      enable-all: true
`

func TestReadGolangciReadsTheDeclaredSubset(t *testing.T) {
	t.Parallel()
	cfg, err := readGolangci(".golangci.yml", []byte(lintConfig))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.path != ".golangci.yml" || cfg.line != 4 || cfg.defaults != "none" || cfg.defaultAt != 3 {
		t.Fatalf("cfg = %+v", cfg)
	}
	if got := namedList(cfg.enable); !slices.Equal(got, []string{"errcheck:6", "govet:6"}) {
		t.Errorf("enable = %v", got)
	}
	if got := namedList(cfg.disable); !slices.Equal(got, []string{"wsl:8"}) {
		t.Errorf("disable = %v", got)
	}
	want := []exclusion{
		{path: `_test\.go`, linters: []string{"funlen", "mnd"}, line: 12},
		{pathExcept: "^internal/", text: "shadows", source: "^// keep", line: 14},
		{path: "third_party/", line: 18},
	}
	if len(cfg.exclusions) != len(want) {
		t.Fatalf("exclusions = %+v", cfg.exclusions)
	}
	for i := range want {
		got := cfg.exclusions[i]
		if got.path != want[i].path || got.pathExcept != want[i].pathExcept || got.text != want[i].text ||
			got.source != want[i].source || !slices.Equal(got.linters, want[i].linters) || got.line != want[i].line {
			t.Errorf("exclusion %d = %+v, want %+v", i, got, want[i])
		}
	}
	if enabled := child(cfg.settings, "govet", "enable-all"); enabled == nil || enabled.Value != "true" {
		t.Errorf("settings = %+v", cfg.settings)
	}
}

func TestReadGolangciDefaultsWhatTheFileLeavesOut(t *testing.T) {
	t.Parallel()
	cfg, err := readGolangci(".golangci.yaml", []byte("version: \"2\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.line != 1 || cfg.defaults != "standard" || cfg.defaultAt != 1 || cfg.enable != nil || cfg.disable != nil || cfg.exclusions != nil {
		t.Fatalf("cfg = %+v", cfg)
	}
	if cfg.settings == nil || len(entries(cfg.settings)) != 0 {
		t.Fatalf("settings of a file without settings = %+v", cfg.settings)
	}
	json, err := readGolangci(".golangci.json", []byte(`{"version": "2", "linters": {"default": "all"}}`))
	if err != nil || json.defaults != "all" {
		t.Fatalf("a JSON configuration = %+v, %v", json, err)
	}
}

func TestReadGolangciNamesEveryConstructOutsideTheSubset(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct{ text, want string }{
		"no version":           {"linters:\n  default: all\n", ".golangci.yml:1: a version other than 2; run golangci-lint migrate"},
		"version 1":            {"version: 1\n", ".golangci.yml:1: a version other than 2; run golangci-lint migrate"},
		"linters scalar":       {"version: \"2\"\nlinters: all\n", ".golangci.yml:2: linters that is not a mapping"},
		"unknown default":      {"version: \"2\"\nlinters:\n  default: every\n", `.golangci.yml:3: linters.default "every"`},
		"enable scalar":        {"version: \"2\"\nlinters:\n  enable: errcheck\n", ".golangci.yml:3: linters.enable that is not a list of names"},
		"disable of mappings":  {"version: \"2\"\nlinters:\n  disable:\n    - {name: wsl}\n", ".golangci.yml:4: linters.disable that is not a list of names"},
		"rules scalar":         {"version: \"2\"\nlinters:\n  exclusions:\n    rules: none\n", ".golangci.yml:4: linters.exclusions.rules that is not a list of rules"},
		"rule scalar":          {"version: \"2\"\nlinters:\n  exclusions:\n    rules: [funlen]\n", ".golangci.yml:4: an exclusion rule that is not a mapping"},
		"rule field list":      {"version: \"2\"\nlinters:\n  exclusions:\n    rules:\n      - path: [a, b]\n", ".golangci.yml:5: an exclusion rule field path that is not a string"},
		"rule linters scalar":  {"version: \"2\"\nlinters:\n  exclusions:\n    rules:\n      - linters: funlen\n", ".golangci.yml:5: an exclusion rule linters that is not a list of names"},
		"paths of mappings":    {"version: \"2\"\nlinters:\n  exclusions:\n    paths:\n      - {path: gen}\n", ".golangci.yml:5: linters.exclusions.paths that is not a list of names"},
		"settings not mapping": {"version: \"2\"\nlinters:\n  settings: [govet]\n", ".golangci.yml:3: linters.settings that is not a mapping"},
	} {
		_, err := readGolangci(".golangci.yml", []byte(tc.text))
		if !errors.Is(err, model.ErrOutsideSubset) || !strings.HasPrefix(err.Error(), tc.want+": ") {
			t.Errorf("%s = %v, want %q", name, err, tc.want)
		}
	}
	_, err := readGolangci(".golangci.yml", []byte("version: \"2\"\nlinters: [\n"))
	if err == nil || errors.Is(err, model.ErrOutsideSubset) || !strings.HasPrefix(err.Error(), "parse .golangci.yml: ") {
		t.Errorf("a syntax error = %v, want a parse error naming the file", err)
	}
}

func TestLocateGolangciFindsTheOneConfiguration(t *testing.T) {
	t.Parallel()
	input := func(files map[string]string) model.Input {
		tree := make(map[string][]byte, len(files))
		for path, content := range files {
			tree[path] = []byte(content)
		}
		return model.Input{Snapshot: proof.NewTree(tree)}
	}
	path, data, found, err := locateGolangci(input(map[string]string{".golangci.yaml": "version: \"2\"\n", "sub/.golangci.yml": "x"}))
	if err != nil || !found || path != ".golangci.yaml" || string(data) != "version: \"2\"\n" {
		t.Fatalf("one configuration = %q, %q, %v, %v", path, data, found, err)
	}
	if path, _, found, err := locateGolangci(input(map[string]string{"sub/.golangci.yml": "x"})); err != nil || found || path != ".golangci.yml" {
		t.Fatalf("no configuration at the root = %q, %v, %v", path, found, err)
	}
	for name, tc := range map[string]struct {
		files map[string]string
		want  string
	}{
		"two files": {map[string]string{".golangci.yml": "a", ".golangci.json": "b"}, ".golangci.json:1: a second golangci-lint configuration beside .golangci.yml"},
		"toml":      {map[string]string{".golangci.toml": "version = \"2\"\n"}, ".golangci.toml:1: a TOML configuration"},
	} {
		if _, _, _, err := locateGolangci(input(tc.files)); !errors.Is(err, model.ErrOutsideSubset) || !strings.HasPrefix(err.Error(), tc.want+": ") {
			t.Errorf("%s = %v, want %q", name, err, tc.want)
		}
	}
	big := model.Input{Snapshot: proof.NewTree(map[string][]byte{".golangci.yml": make([]byte, model.MaxReadBytes+1)})}
	if path, _, found, err := locateGolangci(big); found || path != ".golangci.yml" || !errors.Is(err, model.ErrTooLarge) {
		t.Fatalf("a configuration over the read bound = %q, %v, %v", path, found, err)
	}
}

// namedList renders a list of names with their lines for a comparison.
func namedList(list []named) []string {
	out := make([]string, 0, len(list))
	for _, n := range list {
		out = append(out, fmt.Sprintf("%s:%d", n.name, n.line))
	}
	return out
}

func TestReadGolangciNamesAnExclusionsBlockOutsideTheSubset(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"all", "[generated]"} {
		_, err := readGolangci(".golangci.yml", []byte("version: \"2\"\nlinters:\n  exclusions: "+value+"\n"))
		want := ".golangci.yml:3: linters.exclusions that is not a mapping: "
		if !errors.Is(err, model.ErrOutsideSubset) || !strings.HasPrefix(err.Error(), want) {
			t.Errorf("exclusions %q = %v, want %q", value, err, want)
		}
	}
}

func TestReadGolangciResolvesAliasesAndLeavesOtherKeysUnread(t *testing.T) {
	t.Parallel()
	const content = `version: "2"
name: &name govet
enabled: &enabled [*name]
rule: &rule {path: ^internal/, linters: *enabled}
settings: &settings {govet: {enable-all: true}}
excluded: &excluded {rules: [*rule], paths: [vendor/]}
linters:
  enable: *enabled
  disable: [*name]
  exclusions: *excluded
  settings: *settings
run: [unread]
formatters: unread
`
	cfg, err := readGolangci(".golangci.yml", []byte(content))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.line != 7 || cfg.defaultAt != 7 || cfg.defaults != defaultStandard {
		t.Errorf("default = %+v", cfg)
	}
	for _, list := range [][]named{cfg.enable, cfg.disable} {
		if got := namedList(list); !slices.Equal(got, []string{"govet:2"}) {
			t.Errorf("aliased names = %v", got)
		}
	}
	if len(cfg.exclusions) != 2 {
		t.Fatalf("exclusions = %+v", cfg.exclusions)
	}
	if got := cfg.exclusions[0]; got.path != "^internal/" || got.line != 4 || !slices.Equal(got.linters, []string{"govet"}) {
		t.Errorf("aliased rule = %+v", got)
	}
	if got := cfg.exclusions[1]; got.path != "vendor/" || got.line != 6 || len(got.linters) != 0 {
		t.Errorf("aliased paths = %+v", got)
	}
	if enabled := child(cfg.settings, "govet", "enable-all"); enabled == nil || enabled.Value != "true" {
		t.Errorf("aliased settings = %+v", cfg.settings)
	}
}

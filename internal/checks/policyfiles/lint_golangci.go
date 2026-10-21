package policyfiles

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/model"
)

// The linters.default values of golangci-lint version 2.
const (
	defaultStandard = "standard"
	defaultAll      = "all"
	defaultNone     = "none"
	defaultFast     = "fast"
)

// The keys of the configuration the judgments read more than once.
const (
	keyLinters = "linters"
	keyRules   = "rules"
	keyEnable  = "enable"
	keyDisable = "disable"
)

// golangci is the part of a golangci-lint configuration of version 2 the
// lint checks judge: which linters run, which issues are excluded, and the
// settings node each per-linter rule reads. The declared subset is the
// version key, linters.default, linters.enable, linters.disable, the rules
// and paths of linters.exclusions and linters.settings. Nonempty paths-except
// is a stated limit because it also frees paths from required linters;
// other keys are left unread because no verdict depends on them.
type golangci struct {
	path       string
	line       int         // the line of the linters key, 1 when the key is absent
	defaults   string      // linters.default, standard when absent
	defaultAt  int         // the line of linters.default, the linters line when absent
	enable     []named     // linters.enable, in file order
	disable    []named     // linters.disable, in file order
	exclusions []exclusion // linters.exclusions.rules, then linters.exclusions.paths
	settings   *yaml.Node  // linters.settings, an empty mapping when absent
}

// named is one linter name of a list with the line it sits on.
type named struct {
	name string
	line int
}

// exclusion is one rule of linters.exclusions.rules, or one entry of
// linters.exclusions.paths, which frees its path from every linter.
type exclusion struct {
	path, pathExcept, text, source string
	linters                        []string // empty for every linter
	line                           int
}

// golangciNames are the configuration files golangci-lint reads at the
// root of a repository.
func golangciNames() []string {
	return []string{".golangci.yml", ".golangci.yaml", ".golangci.toml", ".golangci.json"}
}

// locateGolangci reads the one configuration at the root of the snapshot:
// found is false when there is none, and the path is then the conventional
// name a finding points at. Two configurations, of which golangci-lint
// reads one by its own precedence, and a TOML configuration are outside the
// subset.
func locateGolangci(in model.Input) (string, []byte, bool, error) {
	var first string
	var data []byte
	for _, name := range golangciNames() {
		content, found, err := read(in, name)
		if err != nil {
			return name, nil, false, err
		}
		if !found {
			continue
		}
		if first != "" {
			return name, nil, false, outside(name, 1, "a second golangci-lint configuration beside "+first)
		}
		first, data = name, content
	}
	if first == "" {
		return golangciNames()[0], nil, false, nil
	}
	if strings.HasSuffix(first, ".toml") {
		return first, nil, false, outside(first, 1, "a TOML configuration")
	}
	return first, data, true, nil
}

// readGolangci reads a golangci-lint configuration of version 2 with its
// effective exclusions; a version other than "2" is outside the subset,
// because version 1 configurations are read under other rules.
func readGolangci(path string, data []byte) (golangci, error) {
	root, err := decodeYAML(path, data)
	if err != nil {
		return golangci{}, err
	}
	if version := child(root, "version"); version == nil || version.Kind != yaml.ScalarNode || version.Value != "2" {
		return golangci{}, outside(path, lineOf(version), "a version other than 2; run golangci-lint migrate")
	}
	cfg := golangci{path: path, line: 1, defaults: defaultStandard, defaultAt: 1, settings: &yaml.Node{Kind: yaml.MappingNode}}
	linters := child(root, keyLinters)
	if linters == nil {
		return cfg, nil
	}
	if linters.Kind != yaml.MappingNode {
		return golangci{}, outside(path, linters.Line, "linters that is not a mapping")
	}
	cfg.line = keyLine(root, keyLinters)
	cfg.defaultAt = cfg.line
	if err := cfg.readBlock(linters); err != nil {
		return golangci{}, err
	}
	return cfg, nil
}

// readBlock reads the linters block: the lists, the exclusions and the
// settings node.
func (cfg *golangci) readBlock(linters *yaml.Node) error {
	if err := cfg.readLinters(linters); err != nil {
		return err
	}
	exclusions, err := readExclusions(cfg.path, child(linters, "exclusions"))
	if err != nil {
		return err
	}
	cfg.exclusions = exclusions
	if settings := child(linters, "settings"); settings != nil {
		if settings.Kind != yaml.MappingNode {
			return outside(cfg.path, settings.Line, "linters.settings that is not a mapping")
		}
		cfg.settings = settings
	}
	return nil
}

// readLinters reads linters.default, linters.enable and linters.disable.
func (cfg *golangci) readLinters(linters *yaml.Node) error {
	if value := child(linters, "default"); value != nil {
		switch value.Value {
		case defaultStandard, defaultAll, defaultNone, defaultFast:
			cfg.defaults, cfg.defaultAt = value.Value, value.Line
		default:
			return outside(cfg.path, value.Line, fmt.Sprintf("linters.default %q", value.Value))
		}
	}
	var err error
	if cfg.enable, err = names(cfg.path, "linters.enable", child(linters, keyEnable)); err != nil {
		return err
	}
	cfg.disable, err = names(cfg.path, "linters.disable", child(linters, keyDisable))
	return err
}

// readExclusions reads the rules and the paths of linters.exclusions.
func readExclusions(file string, node *yaml.Node) ([]exclusion, error) {
	if node != nil && node.Kind != yaml.MappingNode {
		return nil, outside(file, node.Line, "linters.exclusions that is not a mapping")
	}
	if except := child(node, "paths-except"); except != nil && (except.Kind != yaml.SequenceNode || len(except.Content) != 0) {
		return nil, outside(file, except.Line, "linters.exclusions.paths-except")
	}
	var out []exclusion
	rules := child(node, keyRules)
	if rules != nil && rules.Kind != yaml.SequenceNode {
		return nil, outside(file, rules.Line, "linters.exclusions.rules that is not a list of rules")
	}
	if rules != nil {
		for _, item := range rules.Content {
			rule, err := readRule(file, resolve(item))
			if err != nil {
				return nil, err
			}
			out = append(out, rule)
		}
	}
	paths, err := names(file, "linters.exclusions.paths", child(node, "paths"))
	for _, p := range paths {
		out = append(out, exclusion{path: p.name, line: p.line})
	}
	return out, err
}

// readRule reads one exclusion rule: the four patterns and the linters.
func readRule(file string, node *yaml.Node) (exclusion, error) {
	if node.Kind != yaml.MappingNode {
		return exclusion{}, outside(file, node.Line, "an exclusion rule that is not a mapping")
	}
	rule := exclusion{line: node.Line}
	for _, field := range []struct {
		key    string
		target *string
	}{{"path", &rule.path}, {"path-except", &rule.pathExcept}, {"text", &rule.text}, {"source", &rule.source}} {
		value := child(node, field.key)
		if value != nil && value.Kind != yaml.ScalarNode {
			return exclusion{}, outside(file, value.Line, "an exclusion rule field "+field.key+" that is not a string")
		}
		if value != nil {
			*field.target = value.Value
		}
	}
	linters, err := names(file, "an exclusion rule linters", child(node, "linters"))
	for _, l := range linters {
		rule.linters = append(rule.linters, l.name)
	}
	return rule, err
}

// names reads a list of scalars with their lines; an absent node is no
// list, anything but a list of scalars is outside the subset.
func names(file, what string, node *yaml.Node) ([]named, error) {
	if node == nil {
		return nil, nil
	}
	notList := outside(file, node.Line, what+" that is not a list of names")
	if node.Kind != yaml.SequenceNode {
		return nil, notList
	}
	out := make([]named, 0, len(node.Content))
	for _, item := range node.Content {
		if item = resolve(item); item.Kind != yaml.ScalarNode {
			return nil, outside(file, item.Line, what+" that is not a list of names")
		}
		out = append(out, named{name: item.Value, line: item.Line})
	}
	return out, nil
}

// keyLine answers the line of a key of a mapping, 1 when it is absent.
func keyLine(node *yaml.Node, key string) int {
	for _, e := range entries(node) {
		if e.key == key {
			return e.line
		}
	}
	return 1
}

// lineOf answers the line of a node, 1 for an absent one.
func lineOf(node *yaml.Node) int {
	if node == nil {
		return 1
	}
	return node.Line
}

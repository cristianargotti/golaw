package policyfiles

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/model"
)

const (
	layerFiles         = "files"
	layerAllow         = "allow"
	layerAll           = "$all"
	codeLayerMissing   = "golangci-layer-missing"
	codeLayerOpen      = "golangci-layer-open"
	codeFloat32Allowed = "golangci-float32-allowed"
)

const (
	capabilityHexagonal = "hexagonal"
	goModFile           = "go.mod"
	layerDomain         = "domain"
	layerApplication    = "application"
	layerPrefix         = "internal/"
	layerWildcards      = "*?["
	layerStrict         = "strict"
	layerRulePrefix     = "depguard rule "
)

// A module read failure must retain its path when the configuration is readable.
type layerReadError struct{ cause error }

func (e layerReadError) Error() string { return e.cause.Error() }
func (e layerReadError) Unwrap() error { return e.cause }

type layer struct {
	dir     string
	imports []string
}

func innerLayers() []layer {
	return []layer{
		{dir: layerDomain, imports: []string{layerDomain}},
		{dir: layerApplication, imports: []string{layerDomain, layerApplication}},
	}
}

// Without a declared module, only $all certainly opens an inner layer.
func judgeLayers(in model.Input, cfg golangci) ([]model.Finding, error) {
	if !in.Policy.Has(capabilityHexagonal) {
		return nil, nil
	}
	data, _, err := read(in, goModFile)
	if err != nil {
		return nil, layerReadError{cause: err}
	}
	module := modulePath(data)
	rules, err := layerRulesNode(cfg)
	if err != nil {
		return nil, err
	}
	line := cfg.settingLine(linterDepguard)
	var out []model.Finding
	for _, l := range innerLayers() {
		f, ok, err := judgeLayer(cfg.path, line, entries(rules), l, module)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, f)
		}
	}
	return out, nil
}

// A known strict rule suffices; unresolved alternatives prevent a certain finding.
func judgeLayer(path string, line int, rules []entry, l layer, module string) (model.Finding, bool, error) {
	var first *model.Finding
	var limit error
	for _, rule := range rules {
		covered, why, err := judgeLayerRule(path, rule, l, module)
		switch {
		case err != nil:
			limit = err
		case covered && why == "":
			return model.Finding{}, false, nil
		case covered && first == nil && namesLayer(scalars(child(rule.value, layerFiles)), l.dir):
			f := finding(path, rule.line, codeLayerOpen, why+" in "+path)
			first = &f
		}
	}
	if limit != nil {
		return model.Finding{}, false, limit
	}
	if first != nil {
		return *first, true, nil
	}
	return finding(path, line, codeLayerMissing, fmt.Sprintf(
		"depguard has no strict rule over internal/%s; add the %s rule golaw init renders under linters.settings.depguard.rules of %s", l.dir, l.dir, path)), true, nil
}

// A global rule's fix must not change the imports of unrelated layers.
func namesLayer(files []string, dir string) bool {
	return slices.ContainsFunc(files, func(pattern string) bool {
		covered, _ := selects(pattern, dir)
		return covered && pattern != layerAll && !strings.HasPrefix(pattern, "!")
	})
}

// Unsupported settings matter only when this rule could protect the layer.
func judgeLayerRule(path string, rule entry, l layer, module string) (bool, string, error) {
	if node := resolve(rule.value); node.Kind != yaml.MappingNode {
		return false, "", outside(path, rule.line, layerRulePrefix+rule.key+" that is not a mapping")
	}
	if _, err := names(path, layerRulePrefix+rule.key+" files", child(rule.value, layerFiles)); err != nil {
		return false, "", err
	}
	covered, unsure := covers(scalars(child(rule.value, layerFiles)), l.dir)
	if !covered && !unsure {
		return false, "", nil
	}
	if err := layerRuleLimit(path, rule, l, module); err != nil {
		return false, "", err
	}
	why, open := opened(rule, l, module)
	if unsure && !open {
		return false, "", outside(path, rule.line, layerRulePrefix+rule.key+
			" selects its files with a pattern golaw does not resolve, so it may hold internal/"+l.dir)
	}
	return covered, why, nil
}

// Depguard treats a list with no positive entry as selecting every file.
func covers(files []string, dir string) (bool, bool) {
	positive, unsure := false, false
	for _, f := range files {
		if strings.HasPrefix(f, "!") {
			continue
		}
		positive = true
		covered, maybe := selects(f, dir)
		if covered {
			return true, false
		}
		unsure = unsure || maybe
	}
	return !positive, unsure
}

// Only selectors naming a layer have known coverage; broader globs remain limits.
func selects(pattern, dir string) (bool, bool) {
	if pattern == layerAll || strings.Contains(pattern, layerPrefix+dir+"/") || strings.HasSuffix(pattern, layerPrefix+dir) {
		return true, false
	}
	if _, rest, found := strings.Cut(pattern, layerPrefix); found {
		if segment, _, _ := strings.Cut(rest, "/"); segment != "" && !strings.ContainsAny(segment, layerWildcards) {
			return false, false
		}
	}
	if top, _, _ := strings.Cut(strings.TrimPrefix(pattern, "/"), "/"); top != "internal" && !strings.ContainsAny(top, layerWildcards) {
		return false, false
	}
	return false, pattern != "$test" && strings.ContainsAny(pattern, layerWildcards)
}

// Original mode also restricts imports when its allow list is nonempty.
func restricts(rule entry) bool {
	mode := child(rule.value, "list-mode")
	if mode != nil && strings.EqualFold(mode.Value, layerStrict) {
		return true
	}
	original := mode == nil || mode.Value == "" || strings.EqualFold(mode.Value, "original")
	return original && len(scalars(child(rule.value, layerAllow))) != 0
}

// A layer needs a restrictive rule that permits no outward module imports.
func opened(rule entry, l layer, module string) (string, bool) {
	if !restricts(rule) {
		return fmt.Sprintf("depguard rule %s over internal/%s is not strict; set its list-mode to strict", rule.key, l.dir), true
	}
	for _, allow := range scalars(child(rule.value, layerAllow)) {
		if allow == layerAll || outsideLayer(allow, l, module) {
			return fmt.Sprintf("depguard rule %s lets internal/%s import %s; allow only the standard library and the layers it may import", rule.key, l.dir, allow), true
		}
	}
	return "", false
}

// Third-party imports belong to decision records, so only local imports matter.
func outsideLayer(allow string, l layer, module string) bool {
	allow = strings.TrimSuffix(allow, "$")
	if module == "" || (allow != module && !strings.HasPrefix(allow, module+"/")) {
		return false
	}
	return !slices.ContainsFunc(l.imports, func(dir string) bool {
		inner := module + "/internal/" + dir
		return allow == inner || strings.HasPrefix(allow, inner+"/")
	})
}

// Float32 loses the precision money needs. The wall clock rule belongs to
// goast.wall-clock, so adapters remain free to read the clock.
func judgeFloat32(_ model.Input, cfg golangci) ([]model.Finding, error) {
	if forbidsFloat32(child(cfg.settings, linterForbidigo, "forbid")) {
		return nil, nil
	}
	if err := float32SettingsLimit(cfg); err != nil {
		return nil, err
	}
	return []model.Finding{finding(cfg.path, cfg.settingLine(linterForbidigo), codeFloat32Allowed,
		"forbidigo forbids no float32; add the pattern ^float32$ under linters.settings.forbidigo.forbid of "+cfg.path)}, nil
}

// An invalid regular expression cannot prohibit the identifier.
func forbidsFloat32(forbid *yaml.Node) bool {
	if forbid == nil || forbid.Kind != yaml.SequenceNode {
		return false
	}
	for _, item := range forbid.Content {
		pattern := resolve(item)
		if pattern.Kind == yaml.MappingNode {
			pattern = child(pattern, "pattern")
		}
		if pattern == nil || pattern.Kind != yaml.ScalarNode {
			continue
		}
		if re, err := regexp.Compile(pattern.Value); err == nil && re.MatchString("float32") { // fallback: a pattern that does not compile forbids nothing
			return true
		}
	}
	return false
}

// A wrong shape cannot prove that the configuration has no protecting rule.
func layerRulesNode(cfg golangci) (*yaml.Node, error) {
	for _, keys := range [][]string{{linterDepguard}, {linterDepguard, keyRules}} {
		node := child(cfg.settings, keys...)
		if node != nil && node.Kind != yaml.MappingNode {
			return nil, outside(cfg.path, node.Line, "linters.settings."+strings.Join(keys, ".")+" that is not a mapping")
		}
	}
	node := child(cfg.settings, linterDepguard, keyRules)
	if node == nil {
		node = &yaml.Node{Kind: yaml.MappingNode}
	}
	return node, nil
}

// Deny precedence is outside this reader when it could close an allowed import.
func layerRuleLimit(path string, rule entry, l layer, module string) error {
	mode := child(rule.value, "list-mode")
	if mode != nil && (mode.Kind != yaml.ScalarNode ||
		!slices.Contains([]string{layerStrict, "lax", "original", ""}, strings.ToLower(mode.Value))) {
		return outside(path, mode.Line, layerRulePrefix+rule.key+" has an unsupported list-mode")
	}
	if mode != nil && strings.EqualFold(mode.Value, "lax") {
		return nil
	}
	if _, err := names(path, layerRulePrefix+rule.key+" allow", child(rule.value, layerAllow)); err != nil {
		return err
	}
	deny := child(rule.value, "deny")
	_, open := opened(rule, l, module)
	if restricts(rule) && open && deny != nil && (deny.Kind != yaml.SequenceNode || len(deny.Content) != 0) {
		return outside(path, deny.Line, layerRulePrefix+rule.key+" uses deny precedence that golaw does not resolve")
	}
	return nil
}

// Validate only after no known pattern forbids float32, since extra entries
// cannot undo a match and must not turn a measured configuration into a limit.
func float32SettingsLimit(cfg golangci) error {
	settings := child(cfg.settings, linterForbidigo)
	if settings != nil && settings.Kind != yaml.MappingNode {
		return outside(cfg.path, settings.Line, "linters.settings.forbidigo that is not a mapping")
	}
	forbid := child(settings, "forbid")
	if forbid == nil {
		return nil
	}
	if forbid.Kind != yaml.SequenceNode {
		return outside(cfg.path, forbid.Line, "linters.settings.forbidigo.forbid that is not a list")
	}
	for _, item := range forbid.Content {
		pattern := resolve(item)
		if pattern.Kind == yaml.MappingNode {
			pattern = child(pattern, "pattern")
		}
		if pattern != nil && pattern.Kind != yaml.ScalarNode {
			return outside(cfg.path, pattern.Line, "a forbidigo pattern that is not a string")
		}
	}
	return nil
}

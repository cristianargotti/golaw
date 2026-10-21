package policyfiles

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/model"
)

// The finding codes of policy.golangci.
const (
	codeGolangciMissing = "golangci-missing"
	codeLinterOff       = "golangci-linter-off"
	codeLinterDisabled  = "golangci-linter-disabled"
	codeHollowed        = "golangci-hollowed"
	codeUnlisted        = "golangci-exclusion-unlisted"
	codeShadowOff       = "golangci-shadow-off"
)

// lintScope is the scope a lint check measures: the configuration files.
const lintScope = "1 configuration"

type golangciCheck struct{}

// Golangci is policy.golangci: the golangci-lint configuration runs every
// linter the standard requires under the capabilities, disables none of
// them, frees none of them by an exclusion golaw.yml does not allow, and
// runs the shadow analyzer of govet, because make lint enforces only what
// the configuration enables.
func Golangci() model.Check { return golangciCheck{} }

func (golangciCheck) Spec() model.Spec { return spec("golangci", ruleLinters, "go") }

// Run reads the one configuration at the root and applies every judgment
// of golangciRules to it; a construct outside the declared subset that a
// judgment depends on leaves the check unmeasured with the construct named.
func (golangciCheck) Run(_ context.Context, in model.Input) (model.Result, error) {
	path, data, found, err := locateGolangci(in)
	if err != nil {
		return answer(path, err), nil
	}
	if !found {
		return lintResult([]model.Finding{finding(path, 0, codeGolangciMissing,
			"golangci-lint has no configuration at the root of the repository; write .golangci.yml with the strict set golaw init renders")}), nil
	}
	cfg, err := readGolangci(path, data)
	if err != nil {
		return answer(path, err), nil
	}
	var findings []model.Finding
	for _, judge := range golangciRules() {
		found, err := judge(in, cfg)
		if err != nil {
			if failure, ok := errors.AsType[layerReadError](err); ok {
				return answer(goModFile, failure.cause), nil
			}
			return answer(path, err), nil
		}
		findings = append(findings, found...)
	}
	return lintResult(findings), nil
}

// golangciRules are the judgments of policy.golangci, in report order.
func golangciRules() []func(model.Input, golangci) ([]model.Finding, error) {
	return []func(model.Input, golangci) ([]model.Finding, error){judgeLinters, judgeExclusions, judgeShadow, judgeLayers, judgeFloat32}
}

// lintResult is the measured result of a lint check.
func lintResult(findings []model.Finding) model.Result {
	return model.Result{Status: model.Measured(), Scope: lintScope, Findings: findings}
}

// judgeLinters finds every required linter the configuration disables, at
// the line of its disable entry, and names in one finding the required
// linters it does not enable.
func judgeLinters(in model.Input, cfg golangci) ([]model.Finding, error) {
	var out []model.Finding
	var off []string
	for _, linter := range requiredLinters(in.Policy) {
		if i := slices.IndexFunc(cfg.disable, func(n named) bool { return n.name == linter }); i >= 0 {
			out = append(out, finding(cfg.path, cfg.disable[i].line, codeLinterDisabled, fmt.Sprintf(
				"linter %s is required and listed in linters.disable; remove it from linters.disable of %s", linter, cfg.path)))
			continue
		}
		on, known := cfg.enabled(linter)
		if !known {
			return nil, outside(cfg.path, cfg.defaultAt, "linters.default fast leaves the required linters to the version of golangci-lint")
		}
		if !on {
			off = append(off, linter)
		}
	}
	if len(off) != 0 {
		out = append(out, finding(cfg.path, cfg.line, codeLinterOff, fmt.Sprintf(
			"required linters are not enabled (%s); add them to linters.enable of %s", strings.Join(off, ", "), cfg.path)))
	}
	return out, nil
}

// judgeShadow finds a govet that does not run the shadow analyzer, which
// golangci-lint leaves off unless the settings name it or enable all.
func judgeShadow(_ model.Input, cfg golangci) ([]model.Finding, error) {
	if err := validateShadowSettings(cfg); err != nil {
		return nil, err
	}
	govet := child(cfg.settings, linterGovet)
	enable, disable := scalars(child(govet, keyEnable)), scalars(child(govet, keyDisable))
	all := isTrue(child(govet, "enable-all")) && !isTrue(child(govet, "disable-all"))
	if !slices.Contains(disable, "shadow") && (all || slices.Contains(enable, "shadow")) {
		return nil, nil
	}
	return []model.Finding{finding(cfg.path, cfg.settingLine(linterGovet), codeShadowOff,
		"govet does not run the shadow analyzer; set enable-all: true or add shadow to linters.settings.govet.enable of "+cfg.path)}, nil
}

// validateShadowSettings keeps the shadow judgment to the shapes it reads:
// golangci-lint decodes a string as a one item list and a quoted boolean as
// a boolean, so another shape could run shadow while the judgment reads it
// off, and a stated limit is answered instead of a false finding.
func validateShadowSettings(cfg golangci) error {
	govet := child(cfg.settings, linterGovet)
	if govet == nil {
		return nil
	}
	const prefix = "linters.settings.govet"
	if govet.Kind != yaml.MappingNode {
		return outside(cfg.path, govet.Line, prefix+" that is not a mapping")
	}
	for _, key := range []string{keyEnable, keyDisable} {
		if _, err := names(cfg.path, prefix+"."+key, child(govet, key)); err != nil {
			return err
		}
	}
	for _, key := range []string{"enable-all", "disable-all"} {
		value := child(govet, key)
		if value != nil && (value.Kind != yaml.ScalarNode ||
			(!isTrue(value) && !slices.Contains([]string{"false", "False", "FALSE"}, value.Value))) {
			return outside(cfg.path, value.Line, prefix+"."+key+" that is not a boolean")
		}
	}
	return nil
}

// settingLine answers the line of a key of linters.settings, the line of
// the linters key when the settings do not hold it.
func (cfg *golangci) settingLine(key string) int {
	if child(cfg.settings, key) == nil {
		return cfg.line
	}
	return keyLine(cfg.settings, key)
}

// isTrue answers whether a node is the YAML scalar true, in the three
// spellings YAML 1.2 and golangci-lint read as true.
func isTrue(node *yaml.Node) bool {
	return node != nil && node.Kind == yaml.ScalarNode && slices.Contains([]string{yamlTrue, "True", "TRUE"}, node.Value)
}

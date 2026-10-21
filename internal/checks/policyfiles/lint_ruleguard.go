package policyfiles

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/model"
)

// The finding codes of policy.ruleguard.
const (
	codeRuleguardUnconfigured = "ruleguard-unconfigured"
	codeRuleguardDisabled     = "ruleguard-disabled"
	codeRuleguardMissing      = "ruleguard-rule-missing"
)

// The names gocritic gives its ruleguard check and the tags that run it.
const (
	checkRuleguard = "ruleguard"
	linterGocritic = "gocritic"
	keySettings    = "settings"
	keyEnableAll   = "enable-all"
	keyDisableAll  = "disable-all"
)

// byteCut is a ruleguard pattern that cuts a value by a count from its
// start, whatever the variables are named.
var byteCut = regexp.MustCompile(`^\s*\$\w+\s*\[\s*:\s*\$\w+\s*\]\s*$`)

type ruleguardCheck struct{}

// Ruleguard is policy.ruleguard: gocritic runs its ruleguard check with a
// rules file the configuration names, and the file holds the byte cut rule
// of the standard, because no linter golangci-lint ships carries it. A
// go.mod without the dsl the rules import stops make lint with the import
// named, and golaw init proposes the requirement, so it is no finding here.
func Ruleguard() model.Check { return ruleguardCheck{} }

func (ruleguardCheck) Spec() model.Spec { return spec("ruleguard", ruleLinters, "go") }

// Run reads the golangci configuration, then the rules files it names; a
// missing configuration is policy.golangci's finding.
func (ruleguardCheck) Run(_ context.Context, in model.Input) (model.Result, error) {
	config, data, found, err := locateGolangci(in)
	if err != nil {
		return answer(config, err), nil
	}
	if !found {
		return lintResult(nil), nil
	}
	cfg, err := readGolangci(config, data)
	if err != nil {
		return answer(config, err), nil
	}
	if err = validateRuleguardSettings(cfg); err != nil {
		return answer(config, err), nil
	}
	gocritic := child(cfg.settings, linterGocritic)
	if !ruleguardRuns(gocritic) {
		return lintResult([]model.Finding{finding(cfg.path, cfg.settingLine(linterGocritic), codeRuleguardDisabled,
			"the ruleguard check of gocritic does not run; keep style in linters.settings.gocritic.enabled-tags of "+cfg.path+" and leave ruleguard out of disabled-checks and style and experimental out of disabled-tags")}), nil
	}
	files, err := rulesFiles(cfg.path, child(gocritic, keySettings, checkRuleguard, keyRules))
	if err != nil {
		return answer(cfg.path, err), nil
	}
	if len(files) == 0 {
		return lintResult([]model.Finding{finding(cfg.path, cfg.settingLine(linterGocritic), codeRuleguardUnconfigured,
			"the gocritic ruleguard setting names no rules file; set rules: '${base-path}/scripts/lint/rules.go' under linters.settings.gocritic.settings.ruleguard of "+cfg.path)}), nil
	}
	findings, file, err := judgeRules(in, cfg.path, files)
	if err != nil {
		return answer(file, err), nil
	}
	return lintResult(findings), nil
}

// ruleguardRuns answers whether gocritic runs its ruleguard check: every
// check, the check named, or the style or experimental tag, and never when
// the check is disabled; the default checks leave it out.
func ruleguardRuns(gocritic *yaml.Node) bool {
	disabled := scalars(child(gocritic, "disabled-tags"))
	if slices.Contains(scalars(child(gocritic, "disabled-checks")), checkRuleguard) ||
		slices.Contains(disabled, "style") || slices.Contains(disabled, "experimental") {
		return false
	}
	all := isTrue(child(gocritic, keyEnableAll)) && !isTrue(child(gocritic, keyDisableAll))
	if all || slices.Contains(scalars(child(gocritic, "enabled-checks")), checkRuleguard) {
		return true
	}
	tags := scalars(child(gocritic, "enabled-tags"))
	return slices.Contains(tags, "style") || slices.Contains(tags, "experimental")
}

// validateRuleguardSettings keeps coercions outside the subset, because
// golangci-lint can enable a check from shapes these node readers ignore.
func validateRuleguardSettings(cfg golangci) error {
	node, prefix := cfg.settings, "linters.settings"
	for _, key := range []string{linterGocritic, keySettings, checkRuleguard} {
		node, prefix = child(node, key), prefix+"."+key
		if node != nil && node.Kind != yaml.MappingNode {
			return outside(cfg.path, node.Line, prefix+" that is not a mapping")
		}
	}
	gocritic := child(cfg.settings, linterGocritic)
	prefix = "linters.settings.gocritic."
	for _, key := range []string{"enabled-checks", "disabled-checks", "enabled-tags", "disabled-tags"} {
		if _, err := names(cfg.path, prefix+key, child(gocritic, key)); err != nil {
			return err
		}
	}
	for _, key := range []string{keyEnableAll, keyDisableAll} {
		value := child(gocritic, key)
		if value != nil && (value.Kind != yaml.ScalarNode ||
			(!isTrue(value) && !slices.Contains([]string{"false", "False", "FALSE"}, value.Value))) {
			return outside(cfg.path, value.Line, prefix+key+" that is not a boolean")
		}
	}
	return nil
}

// rulesFiles reads the comma separated rules setting into repository
// paths: the base-path and config-path variables are the root, where the
// configuration lives. Globs and native separators need resolution this
// reader cannot prove, so they are stated limits along with other variables
// and paths outside the repository.
func rulesFiles(config string, node *yaml.Node) ([]string, error) {
	if node == nil {
		return nil, nil
	}
	if node.Kind != yaml.ScalarNode {
		return nil, outside(config, node.Line, "ruleguard rules that is not a string")
	}
	if strings.TrimSpace(node.Value) == "" {
		return nil, nil
	}
	var out []string
	for raw := range strings.SplitSeq(node.Value, ",") {
		name, err := ruleguardPath(config, node.Line, raw)
		if err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, nil
}

func ruleguardPath(config string, line int, raw string) (string, error) {
	name := strings.TrimSpace(raw)
	for _, prefix := range []string{"${base-path}/", "${config-path}/"} {
		name = strings.TrimPrefix(name, prefix)
	}
	if strings.Contains(name, "${") {
		return "", outside(config, line, "a ruleguard rules path with a variable other than base-path or config-path")
	}
	if strings.ContainsAny(name, "*?[") {
		return "", outside(config, line, "a ruleguard rules path with a glob")
	}
	if strings.ContainsRune(name, '\\') {
		return "", outside(config, line, "a ruleguard rules path with native separators")
	}
	name = path.Clean(name)
	if path.IsAbs(name) || name == ".." || strings.HasPrefix(name, apiParentPrefix) || makeDriveQualified(name) {
		return "", outside(config, line, "a ruleguard rules path outside the repository")
	}
	return name, nil
}

// judgeRules reads every rules file: a missing one is a finding, one that
// does not parse leaves the check unmeasured, and when none holds the byte
// cut rule the first file is named. The file of an error is answered with
// it, so the check names where it stopped.
func judgeRules(in model.Input, config string, files []string) ([]model.Finding, string, error) {
	var out []model.Finding
	var limit error
	var limitFile string
	rule := false
	for _, name := range files {
		data, found, err := read(in, name)
		if err != nil {
			return nil, name, err
		}
		if !found {
			out = append(out, finding(name, 0, codeRuleguardMissing, "the ruleguard rules file "+name+" that "+config+
				" names does not exist; write the stringByteCut rule golaw init renders there"))
			continue
		}
		holds, err := holdsByteCut(name, data)
		if errors.Is(err, model.ErrOutsideSubset) {
			limit, limitFile = err, name
			continue
		}
		if err != nil {
			return nil, name, err
		}
		rule = rule || holds
	}
	if !rule && limit != nil {
		return nil, limitFile, limit
	}
	if !rule && len(out) == 0 {
		out = append(out, finding(files[0], 0, codeRuleguardMissing, "the ruleguard rules of "+strings.Join(files, ", ")+
			" hold no byte cut rule; add the stringByteCut rule golaw init renders"))
	}
	return out, "", nil
}

// holdsByteCut recognizes literal patterns only. A constant expression can
// carry the rule too, so it is a stated limit when no literal proves it.
func holdsByteCut(name string, data []byte) (bool, error) {
	positions := token.NewFileSet()
	file, err := parser.ParseFile(positions, name, data, parser.SkipObjectResolution)
	if err != nil {
		return false, parseError(name, err)
	}
	found := false
	var unknown token.Pos
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || found {
			return !found
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Match" {
			found = slices.ContainsFunc(call.Args, isByteCut)
			if at := nonliteralPattern(call.Args); at.IsValid() {
				unknown = at
			}
		}
		return !found
	})
	if !found && unknown.IsValid() {
		return false, outside(name, positions.Position(unknown).Line, "a ruleguard Match pattern that is not a string literal")
	}
	return found, nil
}

func nonliteralPattern(args []ast.Expr) token.Pos {
	for _, arg := range args {
		lit, ok := arg.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return arg.Pos()
		}
	}
	return token.NoPos
}

// isByteCut answers whether an argument is a string literal of a byte cut.
func isByteCut(arg ast.Expr) bool {
	lit, ok := arg.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return false
	}
	text, err := strconv.Unquote(lit.Value)
	return err == nil && byteCut.MatchString(text)
}

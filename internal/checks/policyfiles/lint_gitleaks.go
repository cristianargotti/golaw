package policyfiles

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// codeGeneratedAllowlist is the finding code of policy.gitleaks-generic.
const codeGeneratedAllowlist = "gitleaks-generated-allowlist"

// gitleaksFile is the gitleaks configuration at the root of a repository.
const gitleaksFile = ".gitleaks.toml"

// gitleaksSettings is the part of a gitleaks configuration the check reads:
// where it extends another configuration and every allowlist by path, the
// global ones of both spellings and those of each rule. Conditions and rule
// scopes can prevent a path from excluding a generated file by itself, so
// they must be retained even though their evaluation is outside the subset.
type gitleaksSettings struct {
	Extend struct {
		Path       string `toml:"path"`
		URL        string `toml:"url"`
		UseDefault bool   `toml:"useDefault"`
	} `toml:"extend"`
	Allowlist  *gitleaksAllowlist  `toml:"allowlist"`
	Allowlists []gitleaksAllowlist `toml:"allowlists"`
	Rules      []struct {
		ID         string              `toml:"id"`
		Path       string              `toml:"path"`
		Allowlist  *gitleaksAllowlist  `toml:"allowlist"`
		Allowlists []gitleaksAllowlist `toml:"allowlists"`
	} `toml:"rules"`
}

// gitleaksAllowlist retains the criteria that can make a path insufficient
// to exempt a file, so a conditional exemption never becomes a finding.
type gitleaksAllowlist struct {
	Paths       []string `toml:"paths"`
	Condition   string   `toml:"condition"`
	Regexes     []string `toml:"regexes"`
	Commits     []string `toml:"commits"`
	Stopwords   []string `toml:"stopwords"`
	TargetRules []string `toml:"targetRules"`
	scope       string
}

// generatedSamples are paths of generated code as the templates and the
// common generators write them; an allowlist path that matches one frees
// generated files from the secret scan.
func generatedSamples() []string {
	return []string{
		"gen/client.gen.go", "internal/adapters/db/sqlcgen/queries.sql.go", "test/mocks/mock_store.go",
		"api/server_gen.go", "internal/proto/service.pb.go", "internal/model/zz_generated.deepcopy.go",
		"sqlcgen/queries.sql.go",
	}
}

type gitleaksGeneric struct{}

// GitleaksGeneric is policy.gitleaks-generic: the gitleaks configuration
// frees no generated file from the secret scan, because a credential
// written into generated code is as live as any other. The local rungs stay
// the developer's to arm or trim, so no lefthook.yml is read here: the
// pipeline's secret scan is the secrets target policy.makefile requires.
func GitleaksGeneric() model.Check { return gitleaksGeneric{} }

func (gitleaksGeneric) Spec() model.Spec { return spec("gitleaks-generic", ruleSecurity) }

// Run judges the gitleaks configuration when the snapshot holds it.
func (gitleaksGeneric) Run(_ context.Context, in model.Input) (model.Result, error) {
	allowlist, err := judgeAllowlists(in)
	if err != nil {
		return answer(gitleaksFile, err), nil
	}
	return model.Result{Status: model.Measured(), Scope: "1 configuration", Findings: allowlist}, nil
}

// judgeAllowlists finds every allowlist path of .gitleaks.toml that
// matches generated code; a configuration that extends another one names
// allowlists the snapshot path does not show, a stated limit.
func judgeAllowlists(in model.Input) ([]model.Finding, error) {
	data, found, err := read(in, gitleaksFile)
	if err != nil || !found {
		return nil, err
	}
	var config gitleaksSettings
	if err := decodeTOML(gitleaksFile, data, &config); err != nil {
		return nil, err
	}
	if config.Extend.Path != "" || config.Extend.URL != "" {
		return nil, outside(gitleaksFile, 1, "an [extend] path or url, whose allowlists golaw does not read")
	}
	return judgeGitleaksPaths(in, configuredAllowlists(config))
}

func configuredAllowlists(config gitleaksSettings) []gitleaksAllowlist {
	lists := appendAllowlists(nil, config.Allowlist, config.Allowlists, "")
	ids := make(map[string]int)
	for _, rule := range config.Rules {
		ids[rule.ID]++
	}
	for _, rule := range config.Rules {
		var scope string
		if rule.Path != "" {
			scope = "a rule path constraint, whose interaction with allowlists golaw does not evaluate"
		}
		if config.Extend.UseDefault && inheritedRulePath(rule.ID) {
			scope = "an inherited rule path constraint, whose interaction with allowlists golaw does not evaluate"
		}
		if rule.ID != "" && ids[rule.ID] > 1 {
			scope = "duplicate rule IDs, whose effective allowlists golaw does not resolve"
		}
		lists = appendAllowlists(lists, rule.Allowlist, rule.Allowlists, scope)
	}
	return lists
}

// inheritedRulePath names the default rules of gitleaks 8.30.1, the pin the
// runner installs, that carry a path constraint: a local rule of the same id
// keeps that path when it extends the defaults, and the path is not in the
// snapshot, so an allowlist under it may free no generated file at all.
func inheritedRulePath(id string) bool {
	switch id {
	case "freemius-secret-key", "hashicorp-tf-password", "kubernetes-secret-yaml", "nuget-config-password", "pkcs12-file":
		return true
	default:
		return false
	}
}

func appendAllowlists(out []gitleaksAllowlist, legacy *gitleaksAllowlist, current []gitleaksAllowlist, scope string) []gitleaksAllowlist {
	if legacy != nil && len(current) > 0 {
		scope = "legacy and current allowlists together, whose precedence golaw does not resolve"
	}
	if legacy != nil {
		copyOf := *legacy
		copyOf.scope = scope
		out = append(out, copyOf)
	}
	for i := range current {
		list := current[i]
		list.scope = scope
		out = append(out, list)
	}
	return out
}

func judgeGitleaksPaths(in model.Input, lists []gitleaksAllowlist) ([]model.Finding, error) {
	tree := generatedTree(in, lists)
	var out []model.Finding
	for i := range lists {
		list := &lists[i]
		for _, pattern := range list.Paths {
			if sample, ok := generatedMatch(tree, pattern); ok {
				if err := allowlistLimit(list); err != nil {
					return nil, err
				}
				out = append(out, finding(gitleaksFile, 0, codeGeneratedAllowlist, fmt.Sprintf(
					"allowlist path %q of %s matches the generated file %s, so a secret written there goes unscanned; remove it, because generated code is scanned like any other",
					pattern, gitleaksFile, sample)))
			}
		}
	}
	return out, nil
}

// allowlistLimit is consulted only after a path matches generated code:
// unrelated configurations do not need the fields this check leaves unread.
func allowlistLimit(list *gitleaksAllowlist) error {
	if list.scope != "" {
		return outside(gitleaksFile, 1, list.scope)
	}
	if len(list.TargetRules) > 0 {
		return outside(gitleaksFile, 1, "allowlist targetRules, whose effective rules golaw does not resolve")
	}
	switch strings.ToUpper(list.Condition) {
	case "", "OR":
		return nil
	case "AND":
		if len(list.Regexes)+len(list.Commits)+len(list.Stopwords) > 0 {
			return outside(gitleaksFile, 1, "an AND allowlist with criteria beyond paths, whose conjunction golaw does not evaluate")
		}
		return nil
	default:
		return outside(gitleaksFile, 1, "an allowlist condition other than OR or AND")
	}
}

// generatedMatch answers the first generated sample an allowlist path
// matches, then the first generated file of the tree it matches, so a tree
// of the repository's own no sample spells is judged too. Validation of
// malformed expressions belongs to gitleaks.
func generatedMatch(tree []string, pattern string) (string, bool) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", false
	}
	if sample, ok := sampleMatch(re); ok {
		return sample, true
	}
	for _, path := range tree {
		if re.MatchString(path) {
			return path, true
		}
	}
	return "", false
}

// generatedTree answers the files of the snapshot whose header marks them
// generated and that an allowlist path no sample matches selects, outside
// testdata and vendor, through one selection over every such path: one
// selection per path asked git once per path on each snapshot.
func generatedTree(in model.Input, lists []gitleaksAllowlist) []string {
	paths := unsampledPaths(lists)
	if len(paths) == 0 {
		return nil
	}
	selected := func(name string) bool {
		return !outsideBuild(name) && slices.ContainsFunc(paths, func(re *regexp.Regexp) bool { return re.MatchString(name) })
	}
	var tree []string
	for _, file := range model.Select(in.Snapshot, selected) {
		if file.Class == model.ClassGenerated {
			tree = append(tree, file.Path)
		}
	}
	return tree
}

// unsampledPaths compiles the allowlist paths no generated sample matches;
// a malformed one is left to gitleaks, which validates its configuration.
func unsampledPaths(lists []gitleaksAllowlist) []*regexp.Regexp {
	var paths []*regexp.Regexp
	for i := range lists {
		for _, pattern := range lists[i].Paths {
			re, err := regexp.Compile(pattern)
			if err != nil {
				continue
			}
			if _, sampled := sampleMatch(re); !sampled {
				paths = append(paths, re)
			}
		}
	}
	return paths
}

// sampleMatch answers the first generated sample an expression matches.
func sampleMatch(re *regexp.Regexp) (string, bool) {
	for _, sample := range generatedSamples() {
		if re.MatchString(sample) {
			return sample, true
		}
	}
	return "", false
}

// outsideBuild reports whether a path sits below a testdata or a vendor
// directory, the trees the Go tool and golaw's coverage leave out: an
// allowlist commonly frees the fake credentials of test fixtures there, and
// removing it would fail the secrets target with no repair named.
func outsideBuild(name string) bool {
	dirs := strings.Split(name, "/")
	return slices.ContainsFunc(dirs[:len(dirs)-1], func(dir string) bool { return dir == "testdata" || dir == "vendor" })
}

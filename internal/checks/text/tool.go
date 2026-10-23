package text

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// toolTerms is text.tool-terms: the configured tool names are forbidden
// outside the allowed path prefixes.
type toolTerms struct{}

// ToolTerms is text.tool-terms. The names come from the policy (Terms.Tools)
// and, when the policy names none of its own, from the two product names of
// section 3 of the specification, with the terms of an enrolled profile
// beside them (toolNames); the allowed prefixes come from the policy
// (Terms.AllowPaths) and, when it names none, from the narrow scopes of the
// same section.
func ToolTerms() model.Check { return toolTerms{} }

func (toolTerms) Spec() model.Spec { return spec("text.tool-terms") }

// Run scans every text target outside the allowed prefixes for the names.
func (toolTerms) Run(_ context.Context, in model.Input) (model.Result, error) {
	names, scopes := toolNames(in.Policy), in.Policy.Terms.AllowPaths
	if len(scopes) == 0 {
		scopes = defaultScopes()
	}
	terms := make([]term, 0, len(names))
	for _, name := range names {
		message := fmt.Sprintf("tool name %q outside its allowed paths; the public tree names no tool", name)
		terms = append(terms, term{pattern: wordPattern(name), code: "tool-term", message: message})
	}
	accept := func(f model.File) bool { return anyText(f) && !underAny(f.Path, scopes) }
	var findings []model.Finding
	read, err := walk(in, accept, func(path string, number int, line string) {
		findings = append(findings, matches(terms, path, number, line)...)
	})
	return result(read, err, findings), nil
}

// toolNames is the tool list in force: the policy's own names, or the two
// default names when it names none, beside the terms the enrolled profile
// adds (A1). CompileWith merged those terms into the list, so they never
// count as a list of the policy's own, which would drop the defaults; a
// list made only of the profile's terms reads as none.
func toolNames(p *model.EffectivePolicy) []string {
	var extra []string
	if p.ProfileData != nil {
		extra = p.ProfileData.Terms
	}
	own := slices.DeleteFunc(slices.Clone(p.Terms.Tools), func(name string) bool { return slices.Contains(extra, name) })
	if len(own) == 0 {
		own = defaultTools()
	}
	return slices.Concat(own, extra)
}

// defaultTools are the two product names of section 3, assembled from
// halves so that this file names no tool.
func defaultTools() []string {
	return []string{"cla" + "ude code", "open" + "ai co" + "dex"}
}

// defaultScopes are the narrow scopes of section 3 where the names may
// appear: the agent assets, the pipeline, the documentation, the adapters,
// the installer, the doctor, the acceptance harness and the proof fixtures.
func defaultScopes() []string {
	return []string{
		".github/", "CONTRIBUTING.md", "README.md", "agents/", "docs/",
		"internal/doctor/", "internal/hook/", "internal/install/", "internal/proof/testdata/", "tools/acceptance/",
	}
}

// wordPattern matches a name as whole words, case-insensitively.
func wordPattern(name string) *regexp.Regexp {
	return regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(strings.ToLower(name)) + `\b`)
}

// underAny reports whether a path lies under one of the allowed paths: an
// entry that ends in a slash is a directory and holds what starts with it,
// any other entry is one file, so README.md never allows README.md.bak.
func underAny(path string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasSuffix(p, "/") && strings.HasPrefix(path, p) || path == p {
			return true
		}
	}
	return false
}

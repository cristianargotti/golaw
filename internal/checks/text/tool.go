package text

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// toolTerms is text.tool-terms: the configured tool names are forbidden
// outside the allowed path prefixes.
type toolTerms struct{}

// ToolTerms is text.tool-terms. The names come from the policy (Terms.Tools)
// and, when the policy names none, from the two product names of section 3
// of the specification; the allowed prefixes come from the policy
// (Terms.AllowPaths) and, when it names none, from the narrow scopes of the
// same section.
func ToolTerms() model.Check { return toolTerms{} }

// Spec returns the check specification.
func (toolTerms) Spec() model.Spec { return spec("text.tool-terms") }

// Run scans every text target outside the allowed prefixes for the names.
func (toolTerms) Run(_ context.Context, in model.Input) (model.Result, error) {
	names, scopes := in.Policy.Terms.Tools, in.Policy.Terms.AllowPaths
	if len(names) == 0 {
		names = defaultTools()
	}
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
	files, err := walk(in, accept, func(path string, number int, line string) {
		findings = append(findings, matches(terms, path, number, line)...)
	})
	return result(files, err, findings), nil
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

// underAny reports whether a path starts with one of the prefixes.
func underAny(path string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

package policyfiles

import (
	"fmt"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// allowedOption is the option of policy.golangci that carries the allowed
// exclusions of golaw.yml as "path:linter" strings (policy.Compile).
const allowedOption = "allowed_exclusions"

// everyLinter names what an exclusion without a linter list frees.
const everyLinter = "every linter"

// judgeExclusions finds every exclusion that frees a required linter: one
// without a path frees it everywhere, or everywhere outside a path, and
// hollows it; one with a path frees it there, which only an entry of the
// allowed exclusions of golaw.yml permits. A rule narrowed by the text or
// the source of an issue frees single issues, not the linter, and is left
// to the review.
func judgeExclusions(in model.Input, cfg golangci) ([]model.Finding, error) {
	allowed, _ := in.Options.Strings(allowedOption)
	required := requiredLinters(in.Policy)
	var out []model.Finding
	for _, rule := range cfg.exclusions {
		if f, ok := judgeExclusion(cfg.path, rule, required, allowed); ok {
			out = append(out, f)
		}
	}
	return out, nil
}

// judgeExclusion judges one exclusion against the required linters and
// the allowed "path:linter" entries.
func judgeExclusion(path string, rule exclusion, required, allowed []string) (model.Finding, bool) {
	if rule.text != "" || rule.source != "" {
		return model.Finding{}, false
	}
	freed := required
	if len(rule.linters) != 0 {
		freed = slices.DeleteFunc(slices.Clone(rule.linters), func(l string) bool { return !slices.Contains(required, l) })
	}
	if len(freed) == 0 {
		return model.Finding{}, false
	}
	if rule.path == "" {
		where := "everywhere"
		if rule.pathExcept != "" {
			where = fmt.Sprintf("outside %q", rule.pathExcept)
		}
		return finding(path, rule.line, codeHollowed, fmt.Sprintf(
			"an exclusion rule frees %s %s; remove it from linters.exclusions of %s", freedNames(rule, freed), where, path)), true
	}
	unlisted := slices.DeleteFunc(slices.Clone(freed), func(l string) bool { return slices.Contains(allowed, rule.path+":"+l) })
	if len(unlisted) == 0 {
		return model.Finding{}, false
	}
	return finding(path, rule.line, codeUnlisted, fmt.Sprintf(
		"an exclusion frees %s under %q, which checks.lint.allowed_exclusions of golaw.yml does not list; remove it from linters.exclusions of %s",
		freedNames(rule, unlisted), rule.path, path)), true
}

// freedNames names the required linters a rule frees, or every linter for
// a rule without a linter list.
func freedNames(rule exclusion, linters []string) string {
	if len(rule.linters) == 0 {
		return everyLinter
	}
	return strings.Join(linters, ", ")
}

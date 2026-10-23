package bootstrap

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

// Exclude leaves the destinations a profile assigns to another owner out
// of the plan (A1, at init only): every write, proposal, managed record,
// local edit, CODEOWNERS notice and irregular notice under one of the
// prefixes, where a prefix ending in a slash covers its tree and any other
// names one file, moves into Excluded, and the frozen inputs forget it, so
// the plan neither writes nor records nor names otherwise a file the
// profile leaves out. golaw.yml is never excluded; Excluded stays sorted
// and each destination once.
func (plan *TemplatePlan) Exclude(prefixes []string) {
	if len(prefixes) == 0 {
		return
	}
	var excluded []string
	keep := func(name string) bool {
		if name == PolicyFile || !slices.ContainsFunc(prefixes, func(prefix string) bool { return covers(prefix, name) }) {
			return true
		}
		excluded = append(excluded, name)
		return false
	}
	plan.Changes = kept(plan.Changes, func(c install.Change) string { return c.Path }, keep)
	plan.Proposed = kept(plan.Proposed, func(c install.Change) string { return c.Path }, keep)
	plan.Managed = kept(plan.Managed, func(m policy.ManagedFile) string { return m.Path }, keep)
	plan.LocalEdits = kept(plan.LocalEdits, func(e LocalEdit) string { return e.Path }, keep)
	plan.Ownership.Protected = kept(plan.Ownership.Protected, func(p string) string { return p }, keep)
	plan.Irregular = kept(plan.Irregular, func(p string) string { return p }, keep)
	plan.inputs = kept(plan.inputs, func(c install.Change) string { return c.Path }, keep)
	if len(excluded) > 0 {
		merged := slices.Concat(plan.Excluded, excluded)
		slices.Sort(merged)
		plan.Excluded = slices.Compact(merged)
	}
}

// covers reports whether a prefix names a destination: its tree for a
// prefix ending in a slash, the one file otherwise.
func covers(prefix, name string) bool {
	if strings.HasSuffix(prefix, "/") {
		return strings.HasPrefix(name, prefix)
	}
	return name == prefix
}

// kept answers the items whose name keep accepts, in a new slice, so a
// copy of the plan never sees its lists change.
func kept[T any](items []T, name func(T) string, keep func(string) bool) []T {
	out := make([]T, 0, len(items))
	for _, item := range items {
		if keep(name(item)) {
			out = append(out, item)
		}
	}
	return out
}

// writeExcluded names in one line, in the language of the plan, what the
// profile leaves to another owner.
func writeExcluded(out io.Writer, plan TemplatePlan) error {
	if len(plan.Excluded) == 0 {
		return nil
	}
	label := "Deixados para a plataforma"
	if plan.Language != "" && plan.Language != model.DefaultLanguage {
		label = "Left to the platform"
	}
	if _, err := fmt.Fprintf(out, "%s: %s.\n", label, strings.Join(plan.Excluded, ", ")); err != nil {
		return fmt.Errorf("bootstrap: write excluded notice: %w", err)
	}
	return nil
}

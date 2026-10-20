package policy

import (
	"fmt"
	"slices"
	"time"

	"github.com/cristianargotti/golaw/internal/model"
)

// The check ids under which the coverage and lint blocks travel as options
// to the checks of later milestones.
const (
	CoverageCheck = "coverage.floors"
	LintCheck     = "policy.golangci"
)

// Compile builds the effective policy of a file over the core defaults:
// the floors as written, the terms sorted, the exceptions that have not
// expired by now, the options per check id, the ladder and every waiver,
// with the digest of the file. The enrollment is left unenrolled; Load
// fills it from the record.
func Compile(f File, now time.Time, schemas Schemas) (*model.EffectivePolicy, error) {
	if err := f.Validate(schemas); err != nil {
		return nil, err
	}
	p := model.CoreDefaults()
	p.Digest = Digest(f)
	p.KitVersion = f.Kit.Version
	if f.Profile != nil {
		p.Profile = f.Profile.Name
	}
	if f.Language != nil {
		p.Language = *f.Language
	}
	p.Capabilities = sortedUnique(f.Capabilities)
	p.Sizes = f.Checks.Sizes.floors()
	p.Terms = f.Checks.Terms.policy()
	p.Shell = f.Checks.Shell.policy()
	p.Exceptions = f.exceptions(now)
	p.Options = f.Checks.options(schemas)
	p.Ladder = f.Ladder.ladder()
	p.Waivers = f.waivers()
	return &p, nil
}

// floors answers the size floors, the core defaults where unset.
func (s *Sizes) floors() model.SizeFloors {
	out := model.SizeFloors{FileLines: model.DefaultFileLines, FunctionLines: model.DefaultFunctionLines}
	if s == nil {
		return out
	}
	if s.FileLines != nil {
		out.FileLines = *s.FileLines
	}
	if s.FunctionLines != nil {
		out.FunctionLines = *s.FunctionLines
	}
	return out
}

// policy answers the term policy with both lists sorted and unique.
func (t *Terms) policy() model.TermPolicy {
	if t == nil {
		return model.TermPolicy{Tools: []string{}, AllowPaths: []string{}}
	}
	return model.TermPolicy{Tools: sortedUnique(t.Tools), AllowPaths: sortedUnique(t.AllowPaths)}
}

// policy answers the shell denials, off where unset.
func (s *Shell) policy() model.ShellPolicy {
	if s == nil {
		return model.ShellPolicy{}
	}
	return model.ShellPolicy{DenyPush: s.DenyPush != nil && *s.DenyPush, DenyPR: s.DenyPR != nil && *s.DenyPR}
}

// exceptions keeps the exceptions whose expiry day has not passed by now.
func (f File) exceptions(now time.Time) []model.Exception {
	out := make([]model.Exception, 0, len(f.Exceptions))
	for _, e := range f.Exceptions {
		if Expired(e.Until, now) {
			continue
		}
		out = append(out, model.Exception{Check: e.Check, Path: e.Path, Record: e.Record})
	}
	return out
}

// Expired reports whether a validated expiry day has passed: an entry is
// in force through the whole of its last day, in UTC.
func Expired(until string, now time.Time) bool {
	day, err := time.Parse(dateLayout, until)
	return err != nil || !now.UTC().Before(day.AddDate(0, 0, 1))
}

// options answers the options per check id: the coverage block under
// CoverageCheck, the lint block under LintCheck and every inline block
// converted through its schema, which Validate already matched.
func (c Checks) options(schemas Schemas) map[string]model.Options {
	out := map[string]model.Options{}
	if c.Coverage != nil {
		out[CoverageCheck] = c.Coverage.options()
	}
	if c.Lint != nil {
		out[LintCheck] = model.Options{"allowed_exclusions": model.StringsValue(c.Lint.entries()...)}
	}
	for id, values := range c.Options {
		resolved := model.Options{}
		for name, raw := range values {
			i := slices.IndexFunc(schemas[id], func(o model.OptionSpec) bool { return o.Name == name })
			if value, err := optionValue(schemas[id][i].Kind, raw); err == nil { // fallback: Validate refused a value of another kind already, so the default stands
				resolved[name] = value
			}
		}
		out[id] = resolved
	}
	return out
}

// options answers the coverage floors that are set, as options.
func (c *Coverage) options() model.Options {
	out := model.Options{}
	for name, v := range map[string]*int{"total": c.Total, "package": c.Package, "file": c.File, "min_statements": c.MinStatements} {
		if v != nil {
			out[name] = model.IntValue(*v)
		}
	}
	if c.FileExemptions != nil {
		out["file_exemptions"] = model.StringsValue(sortedUnique(c.FileExemptions)...)
	}
	return out
}

// entries flattens the allowed exclusions to "path:linter" strings, one
// per linter, sorted, so a later check and the tightening compare sets.
func (l *Lint) entries() []string {
	var out []string
	for _, e := range l.AllowedExclusions {
		for _, linter := range e.Linters {
			out = append(out, fmt.Sprintf("%s:%s", e.Path, linter))
		}
	}
	return sortedUnique(out)
}

// sortedUnique answers a sorted copy without repeats, never nil.
func sortedUnique(items []string) []string {
	out := slices.Clone(items)
	if out == nil {
		out = []string{}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

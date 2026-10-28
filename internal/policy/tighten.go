package policy

import (
	"fmt"
	"maps"
	"slices"
	"strconv"

	"github.com/cristianargotti/golaw/internal/model"
)

// floors is the comparable form of a file: every value with its default
// applied, so an absent key and an explicit default compare equal.
type floors struct {
	sizes        model.SizeFloors
	coverage     [4]int // total, package, file, min_statements
	exemptions   []string
	tools        []string
	allowPaths   []string
	exclusions   []string
	capabilities []string
	waivers      map[string]string // a waiver id to its latest expiry day
	exceptions   map[string]string // "check at path" to its latest expiry day
	profile      string
	source       string
}

// effective answers the comparable form of a file.
func effective(f File) floors {
	out := floors{
		sizes:        f.Checks.Sizes.floors(),
		coverage:     [4]int{DefaultCoverage, DefaultCoverage, DefaultCoverage, DefaultMinStatements},
		exemptions:   []string{},
		tools:        f.Checks.Terms.policy().Tools,
		allowPaths:   f.Checks.Terms.policy().AllowPaths,
		exclusions:   []string{},
		capabilities: sortedUnique(f.Capabilities),
		waivers:      make(map[string]string, len(f.Waivers)),
		exceptions:   make(map[string]string, len(f.Exceptions)),
	}
	if c := f.Checks.Coverage; c != nil {
		for i, v := range []*int{c.Total, c.Package, c.File, c.MinStatements} {
			if v != nil {
				out.coverage[i] = *v
			}
		}
		out.exemptions = sortedUnique(c.FileExemptions)
	}
	if f.Checks.Lint != nil {
		out.exclusions = f.Checks.Lint.entries()
	}
	if f.Profile != nil {
		out.profile, out.source = f.Profile.Name, f.Profile.Source
	}
	for _, w := range f.Waivers {
		latest(out.waivers, w.ID, w.Until)
	}
	for _, e := range f.Exceptions {
		latest(out.exceptions, e.Check+" at "+e.Path, e.Until)
	}
	return out
}

// latest keeps the latest expiry day of an entry. The days are validated
// in the fixed width layout 2006-01-02, so their text order is their date
// order.
func latest(days map[string]string, entry, until string) {
	if until > days[entry] {
		days[entry] = until
	}
}

// CheckTightening refuses a proposed policy that weakens the one in force
// (section 5.3, A2): lengths may only fall, percentages rise, the
// statement minimum fall, the exemption, waiver, exception, allow-path and
// exclusion sets shrink, no waiver or exception expires later than in
// force (A24), the tool and capability sets grow, the profile name and
// source stay, and a per-check option moves only in the direction its
// schema declares. The kit pin, the profile version and digest, the
// language, the layout, the shell switches, the ladder and the managed
// hashes are free.
func CheckTightening(inForce, proposed File, schemas Schemas) error {
	if err := proposed.Validate(schemas); err != nil {
		return err
	}
	was, next := effective(inForce), effective(proposed)
	for _, err := range []error{
		downward(keyFileLines, was.sizes.FileLines, next.sizes.FileLines),
		downward(keyFunctionLines, was.sizes.FunctionLines, next.sizes.FunctionLines),
		upward("checks.coverage.total", was.coverage[0], next.coverage[0]),
		upward("checks.coverage.package", was.coverage[1], next.coverage[1]),
		upward("checks.coverage.file", was.coverage[2], next.coverage[2]),
		downward("checks.coverage.min_statements", was.coverage[3], next.coverage[3]),
		shrinking("checks.coverage.file_exemptions", was.exemptions, next.exemptions),
		shrinking(keyAllowPaths, was.allowPaths, next.allowPaths),
		shrinking("checks.lint.allowed_exclusions", was.exclusions, next.exclusions),
		expiring("vulnerability_waivers", was.waivers, next.waivers),
		expiring("exceptions", was.exceptions, next.exceptions),
		growing("checks.terms.tools", was.tools, next.tools),
		growing(keyCapabilities, was.capabilities, next.capabilities),
		samePin("profile.name", was.profile, next.profile),
		samePin("profile.source", was.source, next.source),
		optionsTighten(inForce.Checks.Options, proposed.Checks.Options, schemas),
		surfaceTightening(inForce, proposed),
	} {
		if err != nil {
			return fmt.Errorf("%w: proposed %s weakens %w", model.ErrPolicy, FileName, err)
		}
	}
	return nil
}

// weakening is one rule a proposed value broke.
type weakening struct {
	key  string
	from string
	to   string
}

func (w *weakening) Error() string { return fmt.Sprintf("%s: %s to %s", w.key, w.from, w.to) }

func downward(key string, was, next int) error {
	if next > was {
		return &weakening{key, strconv.Itoa(was), strconv.Itoa(next)}
	}
	return nil
}

func upward(key string, was, next int) error {
	if next < was {
		return &weakening{key, strconv.Itoa(was), strconv.Itoa(next)}
	}
	return nil
}

func shrinking(key string, was, next []string) error {
	for _, item := range next {
		if !slices.Contains(was, item) {
			return &weakening{key, fmt.Sprint(was), "an added " + item}
		}
	}
	return nil
}

// expiring refuses an entry the proposed file adds and an entry whose
// expiry it moves later: the debt set only shrinks, in scope and in time,
// and a later day would bring an expired entry back into force.
func expiring(key string, was, next map[string]string) error {
	for _, entry := range slices.Sorted(maps.Keys(next)) {
		until, kept := was[entry]
		switch {
		case !kept:
			return &weakening{key, fmt.Sprint(slices.Sorted(maps.Keys(was))), "an added " + entry}
		case next[entry] > until:
			return &weakening{key + "[" + entry + "].until", until, next[entry]}
		}
	}
	return nil
}

func growing(key string, was, next []string) error {
	for _, item := range was {
		if !slices.Contains(next, item) {
			return &weakening{key, fmt.Sprint(was), "without " + item}
		}
	}
	return nil
}

func samePin(key, was, next string) error {
	if was != "" && next != was {
		return &weakening{key, was, fmt.Sprintf("%q", next)}
	}
	return nil
}

// optionsTighten compares every declared option of every check named by
// either file, with the schema default where a file leaves it unset.
func optionsTighten(was, next map[string]map[string]any, schemas Schemas) error {
	ids := slices.Sorted(maps.Keys(was))
	for _, id := range slices.Sorted(maps.Keys(next)) {
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	for _, id := range ids {
		for _, spec := range schemas[id] {
			before := resolve(spec, was[id])
			after := resolve(spec, next[id])
			if err := tightens("checks."+id+"."+spec.Name, spec.Tighten, before, after); err != nil {
				return err
			}
		}
	}
	return nil
}

// resolve answers the value a block sets for an option, or its default.
func resolve(spec model.OptionSpec, block map[string]any) model.OptionValue {
	if raw, ok := block[spec.Name]; ok {
		if v, err := optionValue(spec.Kind, raw); err == nil { // fallback: a value of another kind is judged by Validate, the default stands here
			return v
		}
	}
	return spec.Default
}

// tightens applies one direction to a before and after value.
func tightens(key string, direction model.Tighten, before, after model.OptionValue) error {
	switch direction {
	case model.TightenDown:
		return downward(key, before.Int, after.Int)
	case model.TightenUp:
		return upward(key, before.Int, after.Int)
	case model.TightenShrink:
		return shrinking(key, before.Strings, after.Strings)
	case model.TightenGrow:
		return growing(key, before.Strings, after.Strings)
	case model.TightenNone:
		return nil
	default:
		return nil
	}
}

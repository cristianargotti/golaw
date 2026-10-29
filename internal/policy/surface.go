package policy

import (
	"regexp"
	"strconv"

	"github.com/cristianargotti/golaw/internal/model"
)

// The entries of the surface block every function of this file names.
const (
	keySurfaceRules = "surface.rules"
	keyAnalyzer     = "surface.analyzer."
	keyGlobals      = "surface.scripts.globals"
)

var globalPattern = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)

// rulesRank orders the rule sets from the weakest; an unknown set has none.
func rulesRank(rules string) int {
	switch rules {
	case model.SurfaceParity:
		return 1
	case model.SurfaceStrict:
		return 2
	default:
		return 0
	}
}

// rules answers the rule set a block names, the parity floor when unset.
func (s *Surface) rules() string {
	if s == nil || s.Rules == nil {
		return model.SurfaceParity
	}
	return *s.Rules
}

// validate judges the surface block: a known rule set, a known analyzer
// source, clean relative patterns without repeats, a clean report path and
// globals that are script identifiers. The glob grammar itself is judged
// where a pattern is used, so an unparsable one is Unmeasured, never a
// refused policy.
func (s *Surface) validate() error {
	if s == nil {
		return nil
	}
	if s.Rules != nil && rulesRank(*s.Rules) == 0 {
		return invalid("%s %q is not parity or strict", keySurfaceRules, *s.Rules)
	}
	if err := s.Analyzer.validate(); err != nil {
		return err
	}
	if s.Scripts == nil {
		return nil
	}
	for i, name := range s.Scripts.Globals {
		if !globalPattern.MatchString(name) {
			return invalid("%s[%d] %q is not a script identifier", keyGlobals, i, name)
		}
	}
	return nil
}

func (a *Analyzer) validate() error {
	if a == nil {
		return nil
	}
	if a.From != nil {
		switch *a.From {
		case model.FromAuto, model.FromProfile, model.FromProperties, model.FromPolicy:
		default:
			return invalid("%sfrom %q is not auto, profile, properties or policy", keyAnalyzer, *a.From)
		}
	}
	for name, list := range a.lists() {
		if err := cleanPaths(keyAnalyzer+name, list); err != nil {
			return err
		}
	}
	if a.CoverageReport != nil {
		return relPath(keyAnalyzer+"coverage_report", *a.CoverageReport)
	}
	return nil
}

// lists names every pattern list of the block by its golaw.yml entry.
func (a *Analyzer) lists() map[string][]string {
	return map[string][]string{
		"sources": a.Sources, "tests": a.Tests, "test_inclusions": a.TestInclusions,
		"exclusions": a.Exclusions, "coverage_exclusions": a.CoverageExclusions,
	}
}

// policy compiles the block over the defaults of model.DefaultSurface.
func (s *Surface) policy() model.SurfacePolicy {
	out := model.DefaultSurface()
	out.Rules = s.rules()
	if s == nil {
		return out
	}
	if a := s.Analyzer; a != nil {
		if a.From != nil {
			out.Analyzer.From = *a.From
		}
		if a.CoverageReport != nil {
			out.Analyzer.CoverageReport = *a.CoverageReport
		}
		out.Analyzer.Sources, out.Analyzer.Tests = sortedUnique(a.Sources), sortedUnique(a.Tests)
		out.Analyzer.TestInclusions, out.Analyzer.Exclusions = sortedUnique(a.TestInclusions), sortedUnique(a.Exclusions)
		out.Analyzer.CoverageExclusions = sortedUnique(a.CoverageExclusions)
	}
	if s.Scripts != nil {
		out.Globals = sortedUnique(s.Scripts.Globals)
	}
	return out
}

// canonicalRules is the only part of the block the digest holds: the rules
// floor, and only when the file sets it, so a file without the block keeps
// the digest an enrollment record already holds.
func (s *Surface) canonicalRules() *string {
	if s == nil {
		return nil
	}
	return s.Rules
}

// surfaceTightening refuses a proposed file whose rule set is weaker than
// the one in force; the analyzer and the scripts are a description and
// change freely.
func surfaceTightening(inForce, proposed File) error {
	was, next := inForce.Surface.rules(), proposed.Surface.rules()
	if rulesRank(next) < rulesRank(was) {
		return &weakening{keySurfaceRules, was, next}
	}
	return nil
}

// surface explains the two defaults always and every other entry of the
// block the file sets.
func (e *explainer) surface(p *model.EffectivePolicy, f File) {
	s := f.Surface
	e.set(keySurfaceRules, p.Surface.Rules, s != nil && s.Rules != nil)
	a := (*Analyzer)(nil)
	if s != nil {
		a = s.Analyzer
	}
	e.set(keyAnalyzer+"from", p.Surface.Analyzer.From, a != nil && a.From != nil)
	if a != nil {
		for _, name := range []string{"sources", "tests", "test_inclusions", "exclusions", "coverage_exclusions"} {
			if patterns := a.lists()[name]; patterns != nil {
				e.add(keyAnalyzer+name, list(sortedUnique(patterns)), e.origin)
			}
		}
		if a.CoverageReport != nil {
			e.add(keyAnalyzer+"coverage_report", *a.CoverageReport, e.origin)
		}
	}
	if s != nil && s.Scripts != nil && s.Scripts.Globals != nil {
		e.add(keyGlobals, list(p.Surface.Globals), e.origin)
	}
}

// surfaceSwitches explains the two ladder entries of M6.6.4 the file sets.
func (e *explainer) surfaceSwitches(l *Ladder) {
	for _, rung := range []struct {
		name string
		on   *bool
	}{{"surface_lint", l.SurfaceLint}, {"hook_feedback", l.HookFeedback}} {
		if rung.on != nil {
			e.add("ladder."+rung.name, strconv.FormatBool(*rung.on), e.origin)
		}
	}
}

package surface

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// The files a scope is read from, as Origin names them.
const (
	profileFile = "analyzer.yml"
	policyFile  = "golaw.yml"
)

// defaultCoverMode is the mode go test writes without -covermode.
const defaultCoverMode = "set"

// Resolve chooses the scope in force (M6.3.2, A26). Under from: auto the
// profile's analyzer.yml wins, then sonar-project.properties, then the
// golaw.yml lists, then none; from: profile and from: properties pin one
// source and refuse its absence with ErrScopeSource, while from: policy
// reads golaw.yml, which is present by construction.
func Resolve(in ScopeInput) (Scope, error) {
	switch from := in.Policy.From; from {
	case "", model.FromAuto:
		return auto(in), nil
	case model.FromProfile:
		if in.Profile == nil {
			return Scope{}, profileAbsent(in.Profiled)
		}
		return profileScope(in), nil
	case model.FromProperties:
		if !in.HasProps {
			return Scope{}, fmt.Errorf("%w: surface.analyzer.from is properties and the repository root holds no %s", ErrScopeSource, PropertiesFile)
		}
		return propertiesScope(in), nil
	case model.FromPolicy:
		return policyScope(in), nil
	default:
		return Scope{}, fmt.Errorf("%w: surface.analyzer.from %q is not auto, profile, properties or policy", ErrScopeSource, from)
	}
}

// auto is the precedence of from: auto.
func auto(in ScopeInput) Scope {
	switch {
	case in.Profile != nil:
		return profileScope(in)
	case in.HasProps:
		return propertiesScope(in)
	case describes(in.Policy):
		return policyScope(in)
	}
	return finish(Scope{Source: SourceNone}, in)
}

// profileAbsent names why from: profile has no source.
func profileAbsent(profiled string) error {
	if profiled == "" {
		return fmt.Errorf("%w: surface.analyzer.from is profile and no profile is pinned", ErrScopeSource)
	}
	return fmt.Errorf("%w: surface.analyzer.from is profile and the pinned profile %s carries no %s", ErrScopeSource, profiled, profileFile)
}

// describes reports a golaw.yml block that names any list.
func describes(a model.AnalyzerPolicy) bool {
	return len(a.Sources)+len(a.Tests)+len(a.TestInclusions)+len(a.Exclusions)+len(a.CoverageExclusions) > 0
}

func profileScope(in ScopeInput) Scope {
	s := fromAnalyzer(*in.Profile, model.FromProfile, in.Profiled+"/"+profileFile)
	s.Coverage = in.Profile.Coverage
	return finish(s, in)
}

func policyScope(in ScopeInput) Scope {
	s := fromAnalyzer(in.Policy, model.FromPolicy, policyFile)
	return finish(s, in)
}

// fromAnalyzer is the scope a profile or a golaw.yml block describes.
func fromAnalyzer(a model.AnalyzerPolicy, source, origin string) Scope {
	return Scope{
		Source: source, Origin: origin, Sources: a.Sources, Tests: a.Tests, TestInclusions: a.TestInclusions,
		Exclusions: a.Exclusions, CoverageExclusions: a.CoverageExclusions, CoverageReport: a.CoverageReport,
	}
}

func propertiesScope(in ScopeInput) Scope {
	p := ParseProperties(in.Properties)
	v := p.Values
	s := Scope{
		Source: model.FromProperties, Origin: PropertiesFile, Unmeasured: p.Unmeasured,
		Sources: v[keySources], Tests: v[keyTests], Inclusions: v[keyInclusions], Exclusions: v[keyExclusions],
		TestInclusions: v[keyTestInclusions], TestExclusions: v[keyTestExclusions],
		CoverageExclusions: v[keyCoverageExclusions], CPDExclusions: v[keyCPDExclusions],
	}
	if reports := v[keyCoverageReports]; len(reports) > 0 {
		s.CoverageReport = reports[0]
		if len(reports) > 1 {
			s.Unmeasured = append(s.Unmeasured, "coverage_report: "+strconv.Itoa(len(reports))+" reports are named, golaw reads only the first")
		}
	}
	return finish(s, in)
}

// finish fills the defaults of the policy and keeps only what golaw reads:
// a path list entry that is no relative path, or a pattern ValidPattern
// refuses, leaves its list and joins Unmeasured.
func finish(s Scope, in ScopeInput) Scope {
	if s.CoverageReport == "" {
		s.CoverageReport = in.Policy.CoverageReport
	}
	if s.CoverageReport == "" {
		s.CoverageReport = model.DefaultCoverageReport
	}
	if s.Coverage.CoverMode == "" {
		s.Coverage.CoverMode = defaultCoverMode
	}
	s.Sources = keepPaths("sources", s.Sources, &s.Unmeasured)
	s.Tests = keepPaths("tests", s.Tests, &s.Unmeasured)
	for _, list := range patternLists(&s) {
		*list.entries = keepPatterns(list.name, *list.entries, &s.Unmeasured)
	}
	return s
}

// namedList is one pattern list of a scope with its key.
type namedList struct {
	name    string
	entries *[]string
}

// patternLists are the pattern lists in the order Lines prints them.
func patternLists(s *Scope) []namedList {
	return []namedList{
		{"inclusions", &s.Inclusions},
		{"exclusions", &s.Exclusions},
		{"test_inclusions", &s.TestInclusions},
		{"test_exclusions", &s.TestExclusions},
		{"coverage_exclusions", &s.CoverageExclusions},
		{"cpd_exclusions", &s.CPDExclusions},
	}
}

// keepPaths keeps the entries of a directory or file list golaw reads, in the
// slash form the snapshot uses: ./src/ is src, . is the whole tree.
func keepPaths(name string, entries []string, unmeasured *[]string) []string {
	var out []string
	for _, entry := range entries {
		p := entry
		for strings.HasPrefix(p, "./") {
			p = p[2:]
		}
		if p = strings.TrimSuffix(p, "/"); p == "" {
			p = "."
		}
		if strings.HasPrefix(entry, "/") || strings.ContainsAny(p, `*?\[]{}`) || path.Clean(p) != p || p == ".." || strings.HasPrefix(p, "../") {
			*unmeasured = append(*unmeasured, fmt.Sprintf("%s: %q is not a relative directory or file golaw reads", name, entry))
			continue
		}
		out = append(out, p)
	}
	return out
}

// keepPatterns keeps the entries of a pattern list ValidPattern accepts.
func keepPatterns(name string, entries []string, unmeasured *[]string) []string {
	var out []string
	for _, entry := range entries {
		if err := ValidPattern(entry); err != nil {
			*unmeasured = append(*unmeasured, name+": "+err.Error())
			continue
		}
		out = append(out, entry)
	}
	return out
}

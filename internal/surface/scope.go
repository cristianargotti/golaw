package surface

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// Role is how the remote analyzer reads a path.
type Role int

// The roles of a path under a scope.
const (
	RoleOutside Role = iota // the analyzer reads the path as neither main nor test code
	RoleMain
	RoleTest
)

// PropertiesFile is the scanner properties file at the repository root.
const PropertiesFile = "sonar-project.properties"

// SourceNone is the source of a scope no profile, properties file or
// golaw.yml list describes: the whole tracked tree is main code.
const SourceNone = "none"

// Scope is the analyzer scope in force (M6.3.2): what the remote analyzer
// reads as main code, as test code, and which lines count for coverage. It
// describes the pipeline and never relaxes a core law.
type Scope struct {
	Source             string // model.FromProfile, FromProperties, FromPolicy, or SourceNone
	Origin             string // the file it was read from: "<profile name>/analyzer.yml", "sonar-project.properties", "golaw.yml", empty for none
	Sources            []string
	Tests              []string
	Inclusions         []string
	Exclusions         []string
	TestInclusions     []string
	TestExclusions     []string
	CoverageExclusions []string
	CPDExclusions      []string
	CoverageReport     string
	Coverage           model.CoverProfile
	Unmeasured         []string // the reasons a list or a key could not be read; a caller that depends on it answers Unmeasured
}

// ScopeInput is what Resolve chooses from.
type ScopeInput struct {
	Policy     model.AnalyzerPolicy  // the compiled surface.analyzer block
	Profile    *model.AnalyzerPolicy // the bundle's analyzer.yml, nil when none
	Profiled   string                // the profile name, for Origin
	Properties []byte                // the bytes of sonar-project.properties at the root
	HasProps   bool                  // the root holds sonar-project.properties
}

// ScopeOf resolves the scope of a snapshot under a policy: the profile data
// the policy carries, the properties file read through the snapshot, the
// golaw.yml block. A nil policy is the core default.
func ScopeOf(ctx context.Context, s model.Snapshot, p *model.EffectivePolicy) (Scope, error) {
	if err := ctx.Err(); err != nil {
		return Scope{}, fmt.Errorf("resolve the analyzer scope: %w", err)
	}
	in := ScopeInput{Policy: model.DefaultSurface().Analyzer}
	if p != nil {
		in.Policy = p.Surface.Analyzer
		if d := p.ProfileData; d != nil {
			in.Profile, in.Profiled = d.Analyzer, d.Name
		}
	}
	data, err := s.Read(PropertiesFile)
	switch {
	case err == nil:
		in.Properties, in.HasProps = data, true
	case !errors.Is(err, model.ErrNotInSnapshot):
		return Scope{}, fmt.Errorf("read %s: %w", PropertiesFile, err)
	}
	return Resolve(in)
}

// Role answers how the analyzer reads a path, in Sonar's order: a test
// path when the scope names tests and the path is under one and matches
// the test inclusions outside the test exclusions; otherwise main code
// under the sources, inside the inclusions and outside the exclusions. A
// path both would claim is read as test code, the narrower description.
func (s Scope) Role(name string) Role {
	if len(s.Tests) > 0 && under(s.Tests, name) && included(s.TestInclusions, name) && !matchAny(s.TestExclusions, name) {
		return RoleTest
	}
	if (len(s.Sources) == 0 || under(s.Sources, name)) && included(s.Inclusions, name) && !matchAny(s.Exclusions, name) {
		return RoleMain
	}
	return RoleOutside
}

// Covered reports a path whose lines count for coverage: main code outside
// the coverage exclusions.
func (s Scope) Covered(name string) bool {
	return s.Role(name) == RoleMain && !matchAny(s.CoverageExclusions, name)
}

// under reports a path inside one of the directories or files of a list;
// "." is the whole tree.
func under(paths []string, name string) bool {
	for _, p := range paths {
		if p == "." || name == p || strings.HasPrefix(name, p+"/") {
			return true
		}
	}
	return false
}

// included reports a path an inclusion list admits; no list admits all.
func included(patterns []string, name string) bool {
	return len(patterns) == 0 || matchAny(patterns, name)
}

// matchAny reports a path one pattern matches; Resolve kept only the
// patterns ValidPattern accepts.
func matchAny(patterns []string, name string) bool {
	for _, pattern := range patterns {
		if ok, err := Match(pattern, name); err == nil && ok {
			return true
		}
	}
	return false
}

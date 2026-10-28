package policy_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

// surfacePolicy names every entry of the surface block of M6.3.2.
const surfacePolicy = minimalPolicy + `surface:
  rules: strict
  analyzer:
    from: properties
    sources: [cmd, internal]
    tests: [test]
    test_inclusions: ["**/*_test.go"]
    exclusions: ["**/gen/**", "integration/**"]
    coverage_exclusions: [examples/*]
    coverage_report: build/profile.cov
  scripts:
    globals: [appCommon, React, React]
ladder: {surface_lint: false, hook_feedback: false}
`

// The digests 7044cd1 computes for the two example files: a policy that
// names no surface entry keeps them, so no enrollment record goes stale.
const (
	minimalDigest = "5649276e1f084b3b056b101e8374b298eaf5df497b921ce3c91a902e72ea2478"
	fullDigest    = "af90b1da925a327413811a0273b1a817dfd47f258abceca41ae045683dfdfccf"
)

func TestCompileCarriesTheSurfaceBlock(t *testing.T) {
	t.Parallel()
	p, err := policy.Compile(decode(t, surfacePolicy), now(), nil)
	if err != nil {
		t.Fatal(err)
	}
	got := p.Surface
	if got.Rules != model.SurfaceStrict || got.Analyzer.From != model.FromProperties || got.Analyzer.CoverageReport != "build/profile.cov" {
		t.Errorf("surface = %+v", got)
	}
	lists := map[string][2][]string{
		"sources":             {got.Analyzer.Sources, {"cmd", "internal"}},
		"tests":               {got.Analyzer.Tests, {"test"}},
		"test_inclusions":     {got.Analyzer.TestInclusions, {"**/*_test.go"}},
		"exclusions":          {got.Analyzer.Exclusions, {"**/gen/**", "integration/**"}},
		"coverage_exclusions": {got.Analyzer.CoverageExclusions, {"examples/*"}},
		"globals":             {got.Globals, {"React", "appCommon"}},
	}
	for name, pair := range lists {
		if !slices.Equal(pair[0], pair[1]) {
			t.Errorf("%s = %v, want %v", name, pair[0], pair[1])
		}
	}
	if want := (model.CoverProfile{CoverMode: "set"}); got.Analyzer.Coverage != want {
		t.Errorf("a golaw.yml never sets the coverage profile: %+v", got.Analyzer.Coverage)
	}
	if p.Ladder.SurfaceLint || p.Ladder.HookFeedback {
		t.Errorf("ladder = %+v, want both surface switches off", p.Ladder)
	}
}

func TestCompileWithoutASurfaceBlockHoldsTheDefaults(t *testing.T) {
	t.Parallel()
	p, err := policy.Compile(decode(t, minimalPolicy), now(), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := model.DefaultSurface()
	got := p.Surface
	if got.Rules != want.Rules || got.Analyzer.From != want.Analyzer.From ||
		got.Analyzer.CoverageReport != want.Analyzer.CoverageReport || got.Globals == nil || len(got.Globals) != 0 {
		t.Errorf("surface = %+v, want %+v", got, want)
	}
	if !p.Ladder.SurfaceLint || !p.Ladder.HookFeedback {
		t.Errorf("ladder = %+v, want both surface switches on by default", p.Ladder)
	}
}

func TestValidateRefusesASurfaceBlockOutsideItsGrammar(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		block string
		named string // empty for a near miss that validates
	}{
		"an unknown rule set":                 {block: "surface: {rules: lenient}\n", named: "surface.rules"},
		"an unknown analyzer source":          {block: "surface: {analyzer: {from: remote}}\n", named: "surface.analyzer.from"},
		"an absolute pattern":                 {block: "surface: {analyzer: {exclusions: [/abs/**]}}\n", named: "surface.analyzer.exclusions"},
		"a pattern that leaves the root":      {block: "surface: {analyzer: {sources: [../other]}}\n", named: "surface.analyzer.sources"},
		"an empty pattern":                    {block: "surface: {analyzer: {tests: [\"\"]}}\n", named: "surface.analyzer.tests"},
		"a repeated pattern":                  {block: "surface: {analyzer: {coverage_exclusions: [a/**, a/**]}}\n", named: "surface.analyzer.coverage_exclusions"},
		"an absolute coverage report":         {block: "surface: {analyzer: {coverage_report: /tmp/profile.cov}}\n", named: "surface.analyzer.coverage_report"},
		"a global that is no identifier":      {block: "surface: {scripts: {globals: [\"not a name\"]}}\n", named: "surface.scripts.globals"},
		"an unknown surface entry":            {block: "surface: {lint: true}\n", named: "lint"},
		"parity (near miss)":                  {block: "surface: {rules: parity}\n"},
		"the pipeline exclusions (near miss)": {block: "surface: {analyzer: {exclusions: [\"**/*_test.go\", integration/**, examples/*]}}\n"},
		"a dollar global (near miss)":         {block: "surface: {scripts: {globals: [$, _app]}}\n"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f, err := policy.Decode([]byte(minimalPolicy + tc.block))
			if err == nil {
				err = f.Validate(nil)
			}
			if tc.named == "" {
				if err != nil {
					t.Fatalf("Validate = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, model.ErrPolicy) || !strings.Contains(err.Error(), tc.named) {
				t.Errorf("error = %v, want ErrPolicy naming %s", err, tc.named)
			}
		})
	}
}

func TestTheDigestHoldsOnlyTheRulesFloorOfTheSurfaceBlock(t *testing.T) {
	t.Parallel()
	if got := policy.Digest(decode(t, minimalPolicy)); got != minimalDigest {
		t.Errorf("minimal digest = %s, want %s: an enrollment record went stale", got, minimalDigest)
	}
	if got := policy.Digest(decode(t, fullPolicy)); got != fullDigest {
		t.Errorf("full digest = %s, want %s: an enrollment record went stale", got, fullDigest)
	}
	described := minimalPolicy + "surface: {analyzer: {from: policy, exclusions: [a/**]}, scripts: {globals: [React]}}\nladder: {surface_lint: false}\n"
	if got := policy.Digest(decode(t, described)); got != minimalDigest {
		t.Errorf("a pipeline description changed the digest: %s", got)
	}
	if got := policy.Digest(decode(t, minimalPolicy+"surface: {rules: strict}\n")); got == minimalDigest {
		t.Error("the rules floor is left out of the digest")
	}
}

func TestTheRulesFloorOnlyTightens(t *testing.T) {
	t.Parallel()
	parity := minimalPolicy + "surface: {rules: parity}\n"
	strict := minimalPolicy + "surface:\n  rules: strict\n"
	tests := map[string]struct {
		inForce, proposed string
		weakens           bool
	}{
		"strict to parity":                                {inForce: strict, proposed: parity, weakens: true},
		"strict to no block":                              {inForce: strict, proposed: minimalPolicy, weakens: true},
		"parity to strict (near miss)":                    {inForce: parity, proposed: strict},
		"no block to strict (near miss)":                  {inForce: minimalPolicy, proposed: strict},
		"a widened exclusion list is free (near miss)":    {inForce: strict, proposed: strict + "  analyzer: {exclusions: [a/**]}\n"},
		"a surface switch turned off is free (near miss)": {inForce: strict, proposed: strict + "ladder: {hook_feedback: false}\n"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := policy.CheckTightening(decode(t, tc.inForce), decode(t, tc.proposed), nil)
			if tc.weakens != (err != nil) {
				t.Fatalf("CheckTightening = %v, weakens %v", err, tc.weakens)
			}
			if tc.weakens && (!errors.Is(err, model.ErrPolicy) || !strings.Contains(err.Error(), "surface.rules: strict to parity")) {
				t.Errorf("error = %v, want the surface.rules weakening named", err)
			}
		})
	}
}

func TestExplainNamesTheSurfaceEntries(t *testing.T) {
	t.Parallel()
	root, d := enrolled(t, surfacePolicy)
	l, err := policy.Load(t.Context(), input(root, d))
	if err != nil {
		t.Fatal(err)
	}
	got := lines(policy.Explain(l))
	want := map[string]string{
		"surface.rules":                        "strict",
		"surface.analyzer.from":                "properties",
		"surface.analyzer.sources":             "[cmd, internal]",
		"surface.analyzer.tests":               "[test]",
		"surface.analyzer.test_inclusions":     "[**/*_test.go]",
		"surface.analyzer.exclusions":          "[**/gen/**, integration/**]",
		"surface.analyzer.coverage_exclusions": "[examples/*]",
		"surface.analyzer.coverage_report":     "build/profile.cov",
		"surface.scripts.globals":              "[React, appCommon]",
		"ladder.surface_lint":                  "false",
		"ladder.hook_feedback":                 "false",
	}
	for name, value := range want {
		if line := got[name]; line.Value != value || line.Source != "HEAD:golaw.yml" {
			t.Errorf("%s = %+v, want %s from HEAD:golaw.yml", name, line, value)
		}
	}
	core, coreDirs := enrolled(t, minimalPolicy)
	lc, err := policy.Load(t.Context(), input(core, coreDirs))
	if err != nil {
		t.Fatal(err)
	}
	defaults := lines(policy.Explain(lc))
	for name, value := range map[string]string{"surface.rules": "parity", "surface.analyzer.from": "auto"} {
		if line := defaults[name]; line.Value != value || line.Source != policy.SourceCore {
			t.Errorf("%s = %+v, want %s as the core default", name, line, value)
		}
	}
	if _, listed := defaults["surface.analyzer.exclusions"]; listed {
		t.Error("an analyzer list the file leaves out is listed: explain names only what the file sets beyond the two defaults")
	}
}

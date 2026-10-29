package text_test

import (
	"slices"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/checks/text"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/profiletest"
	"github.com/cristianargotti/golaw/internal/proof"
)

// profiledPolicy compiles a policy file that pins the fixture profile, with
// the terms block given, through CompileWith and the fixture's data, the
// way every load of the binary compiles it (D6).
func profiledPolicy(t *testing.T, terms string) *model.EffectivePolicy {
	t.Helper()
	ref := profiletest.Ref(t)
	f, err := policy.Decode([]byte("golaw: 1\nkit: {version: 0.1.0}\n" +
		"profile: {name: " + ref.Name + ", source: " + ref.Source + ", version: " + ref.Version + ", digest: \"" + ref.Digest + "\"}\n" +
		"checks:\n  terms: " + terms + "\n"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := policy.CompileWith(f, time.Date(2026, time.October, 22, 12, 0, 0, 0, time.UTC), nil, profiletest.Data(t))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// termPaths runs text.tool-terms under a policy and answers the paths it
// reports, sorted.
func termPaths(t *testing.T, p *model.EffectivePolicy, files map[string][]byte) []string {
	t.Helper()
	tree := proof.NewTree(files)
	res, err := text.ToolTerms().Run(t.Context(), model.Input{Mode: model.ModeScan, Policy: p, Snapshot: tree, Targets: tree.Files()})
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(res.Findings))
	for k := range res.Findings {
		f := &res.Findings[k]
		got = append(got, f.Path)
	}
	slices.Sort(got)
	return got
}

// TestToolTermsAddsTheProfileTermsToThePolicyInForce pins A1 for the extra
// terms of a profile: a policy that names no tool keeps the kit's default
// names beside the profile's term, a policy that names its own list keeps
// it with the term added, the allowed paths hold for the term, and a list
// made only of the profile's terms reads as no list of the policy's own.
func TestToolTermsAddsTheProfileTermsToThePolicyInForce(t *testing.T) {
	t.Parallel()
	files := map[string][]byte{
		"internal/a.go": []byte("// the legacyci pipeline\n"),
		"internal/b.go": []byte("// built with " + product + "\n"),
		"internal/c.go": []byte("// ourtool runs here\n"),
		"docs/a.md":     []byte("legacyci\n"),
		"vendor/a.go":   []byte("legacyci\n"),
	}
	cases := []struct {
		name, terms string
		want        []string
	}{
		{"no list of its own", "{tools: [], allow_paths: []}", []string{"internal/a.go", "internal/b.go", "vendor/a.go"}},
		{"a list of its own", "{tools: [ourtool], allow_paths: [vendor/]}", []string{"docs/a.md", "internal/a.go", "internal/c.go"}},
		{"only the profile's term", "{tools: [legacyci], allow_paths: []}", []string{"internal/a.go", "internal/b.go", "vendor/a.go"}},
	}
	for _, tc := range cases {
		p := profiledPolicy(t, tc.terms)
		if !slices.Contains(p.Terms.Tools, "legacyci") || p.ProfileData == nil {
			t.Fatalf("%s: compiled terms = %+v", tc.name, p.Terms)
		}
		if got := termPaths(t, p, files); !slices.Equal(got, tc.want) {
			t.Errorf("%s: reported %v, want %v", tc.name, got, tc.want)
		}
	}
}

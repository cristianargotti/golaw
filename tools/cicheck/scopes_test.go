package cicheck_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/checks/text"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/proof"
)

// sectionThreeScopes are the paths of the kit where the tool names may
// appear: the scopes section 3 lists, the marketplace pointer the layout
// of section 4.1 places at the root, and the policy file whose term list
// names them.
func sectionThreeScopes(t *testing.T) []string {
	t.Helper()
	return []string{
		"." + agentID(t) + "-plugin/", ".github/", "CONTRIBUTING.md", "README.md", "agents/", "docs/", "golaw.yml",
		"internal/doctor/", "internal/hook/", "internal/install/", "internal/proof/testdata/", "tools/acceptance/",
	}
}

// scopeSamples answers a path inside a scope and a sibling outside it: the
// directory with a letter more, or the file with a suffix.
func scopeSamples(scope string) (string, string) {
	if dir, ok := strings.CutSuffix(scope, "/"); ok {
		return dir + "/a.md", dir + "x/a.md"
	}
	return scope, scope + ".bak"
}

// TestKitAllowPathsAreTheScopesOfSection3 holds the kit's own policy to
// those scopes and proves each one with a red sibling: under the compiled
// policy, a tool name inside the scope passes and the same name in the
// sibling path beside it is refused.
func TestKitAllowPathsAreTheScopesOfSection3(t *testing.T) {
	t.Parallel()
	f, err := policy.Decode(kitBytes(t, "golaw.yml"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := policy.Compile(f, time.Now(), nil)
	if err != nil {
		t.Fatal(err)
	}
	scopes := sectionThreeScopes(t)
	if got := slices.Sorted(slices.Values(p.Terms.AllowPaths)); !slices.Equal(got, scopes) {
		t.Fatalf("allow_paths = %v, section 3 = %v", got, scopes)
	}
	files := map[string][]byte{}
	for _, scope := range scopes {
		inside, sibling := scopeSamples(scope)
		files[inside], files[sibling] = []byte(p.Terms.Tools[0]+"\n"), []byte(p.Terms.Tools[0]+"\n")
	}
	tree := proof.NewTree(files)
	res, err := text.ToolTerms().Run(t.Context(), model.Input{Mode: model.ModeScan, Policy: p, Snapshot: tree, Targets: tree.Files()})
	if err != nil {
		t.Fatal(err)
	}
	reported := map[string]bool{}
	for _, finding := range res.Findings {
		reported[finding.Path] = true
	}
	for _, scope := range scopes {
		if inside, sibling := scopeSamples(scope); reported[inside] || !reported[sibling] {
			t.Errorf("%s: inside %s reported %v, sibling %s reported %v", scope, inside, reported[inside], sibling, reported[sibling])
		}
	}
}

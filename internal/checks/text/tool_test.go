package text_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/text"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// product is the first default name, assembled from halves so this file
// names no tool.
const product = "Cla" + "ude Code"

func TestToolTermsDefaults(t *testing.T) {
	t.Parallel()
	res := run(t, text.ToolTerms(), map[string]string{
		"internal/a.go":                      "// built with " + product + "\n",
		"docs/a.md":                          product + " is supported\n",
		"agents/x.md":                        product + "\n",
		"internal/proof/testdata/x/red/a.go": product + "\n",
		"README.md":                          "open" + "ai co" + "dex too\n",
		"internal/b.go":                      "// a code review\n",
	})
	if got := codes(res); !slices.Equal(got, []string{"tool-term"}) {
		t.Fatalf("codes = %v", got)
	}
	f := res.Findings[0]
	if f.Path != "internal/a.go" || f.Line != 1 || f.Column != 15 || !strings.Contains(f.Message, strings.ToLower(product)) {
		t.Errorf("finding = %+v", f)
	}
	if res.Scope != "2 files" {
		t.Errorf("scope = %q", res.Scope)
	}
}

func TestToolTermsFromPolicy(t *testing.T) {
	t.Parallel()
	policy := proof.Policy()
	policy.Terms = model.TermPolicy{Tools: []string{"ourtool"}, AllowPaths: []string{"vendor/"}}
	files := map[string][]byte{
		"internal/a.go": []byte("uses OURTOOL here\n"),
		"vendor/a.go":   []byte("ourtool\n"),
		"docs/a.md":     []byte(product + "\n"),
	}
	tree := proof.NewTree(files)
	res, err := text.ToolTerms().Run(t.Context(), model.Input{Mode: model.ModeScan, Policy: policy, Snapshot: tree, Targets: tree.Files()})
	if err != nil || len(res.Findings) != 1 || res.Findings[0].Path != "internal/a.go" {
		t.Errorf("result = %+v, %v", res, err)
	}
}

// TestToolTermsAllowAFileScopeExactly reads an allowed path that ends in a
// slash as a directory and any other as one file, so README.md lets the
// names into the README and not into README.md.bak, and docs/ into the
// documentation and not into docsx/.
func TestToolTermsAllowAFileScopeExactly(t *testing.T) {
	t.Parallel()
	policy := proof.Policy()
	policy.Terms = model.TermPolicy{Tools: []string{"ourtool"}, AllowPaths: []string{"README.md", "docs/"}}
	files := map[string][]byte{}
	for _, path := range []string{"README.md", "README.md.bak", "docs/a.md", "docsx/a.md"} {
		files[path] = []byte("ourtool\n")
	}
	tree := proof.NewTree(files)
	res, err := text.ToolTerms().Run(t.Context(), model.Input{Mode: model.ModeScan, Policy: policy, Snapshot: tree, Targets: tree.Files()})
	got := make([]string, 0, len(res.Findings))
	for _, f := range res.Findings {
		got = append(got, f.Path)
	}
	if err != nil || !slices.Equal(got, []string{"README.md.bak", "docsx/a.md"}) {
		t.Errorf("reported %v, %v", got, err)
	}
}

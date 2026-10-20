package bootstrap_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
)

func TestOwnershipUsesTheLastMatchingOwners(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, path, owners string
		protected          bool
		unsupported        string
	}{
		{"absent", "golaw.yml", "", false, ""},
		{"catch all", "golaw.yml", "* @core\n", false, ""},
		{"same owner", ".github/dependabot.yml", "* @core\n/.github/ @core\n", false, ""},
		{"foreign directory", ".github/dependabot.yml", "* @core\n/.github/ @platform\n", true, ""},
		{"directory near", ".github-other/dependabot.yml", "* @core\n/.github/ @platform\n", false, ""},
		{
			"later same", ".github/dependabot.yml",
			"* @core\n/.github/ @platform\n/.github/dependabot.yml @core\n", false, "",
		},
		{
			"later foreign", ".github/dependabot.yml",
			"* @core\n/.github/ @core\n/.github/dependabot.yml @platform\n", true, "",
		},
		{"root file", "golaw.yml", "* @core\n/golaw.yml @platform\n", true, ""},
		{"root file near", "docs/golaw.yml", "* @core\n/golaw.yml @platform\n", false, ""},
		{"unanchored extension", ".github/dependabot.yml", "* @core\n*.yml @platform\n", true, ""},
		{"extension near", ".github/dependabot.yaml", "* @core\n*.yml @platform\n", false, ""},
		{"recursive directory", "docs/adr/0001.md", "* @core\n/docs/** @platform\n", true, ""},
		{"recursive near", "manual/adr/0001.md", "* @core\n/docs/** @platform\n", false, ""},
		{"relative recursive", "docs/adr/0001.md", "* @core\ndocs/** @platform\n", true, ""},
		{"relative recursive near", "manual/adr/0001.md", "* @core\ndocs/** @platform\n", false, ""},
		{"unanchored directory", "nested/docs/0001.md", "* @core\ndocs/ @platform\n", true, ""},
		{"unanchored directory near", "nested/docs-old/0001.md", "* @core\ndocs/ @platform\n", false, ""},
		{"single component wildcard", "docs/0001.md", "* @core\n/docs/*.md @platform\n", true, ""},
		{"single component near", "docs/adr/0001.md", "* @core\n/docs/*.md @platform\n", false, ""},
		{
			"different owner set", "golaw.yml",
			"* @core @reviewers\n/golaw.yml @core @platform\n", true, "",
		},
		{
			"equal reordered owner set", "golaw.yml",
			"* @core @reviewers\n/golaw.yml @reviewers @core\n", false, "",
		},
		{
			"negation", ".github/dependabot.yml", "* @core\n!/.github/ @platform\n",
			false, "CODEOWNERS line 2: unsupported negation",
		},
		{"negation near", ".github/dependabot.yml", "* @core\n/.github/ @core\n", false, ""},
		{
			"bracket", ".github/dependabot.yml", "* @core\n/.github/[ab].yml @platform\n",
			false, "CODEOWNERS line 2: unsupported bracket pattern",
		},
		{"bracket near", ".github/dependabot.yml", "* @core\n/.github/a.yml @platform\n", false, ""},
		{
			"supported beside unsupported", ".github/dependabot.yml", "* @core\n!/docs/ @platform\n/.github/ @platform\n",
			true, "CODEOWNERS line 2: unsupported negation",
		},
		{
			"unrelated beside unsupported", "golaw.yml", "* @core\n/.github/[ab].yml @platform\n",
			false, "CODEOWNERS line 2: unsupported bracket pattern",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := bootstrap.AllowedPaths([]string{tc.path}, []byte(tc.owners))
			if err != nil {
				t.Fatal(err)
			}
			named := slices.ContainsFunc(got.Unsupported, func(line string) bool { return strings.Contains(line, tc.unsupported) })
			if (tc.unsupported != "") != named || (tc.unsupported == "") != (len(got.Unsupported) == 0) {
				t.Fatalf("unsupported = %v, want %q", got.Unsupported, tc.unsupported)
			}
			wantAllowed, wantProtected := []string{tc.path}, []string(nil)
			if tc.protected {
				wantAllowed, wantProtected = nil, []string{tc.path}
			}
			if !slices.Equal(got.Allowed, wantAllowed) || !slices.Equal(got.Protected, wantProtected) {
				t.Fatalf("allowed = %v, protected = %v", got.Allowed, got.Protected)
			}
		})
	}
}

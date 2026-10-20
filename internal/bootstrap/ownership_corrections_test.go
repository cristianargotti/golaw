package bootstrap_test

import (
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
)

func TestOwnershipWildcardDirectoriesProtectTheirDescendants(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ pattern, owned, near string }{
		{"/doc*", "docs/adr/0001.md", "manual/adr/0001.md"},
		{"test*", "test/deadcode/record.go", "spec/deadcode/record.go"},
		{"**/deadcode", "test/deadcode/internal/record.go", "test/deadcode-other/internal/record.go"},
		{"/.github/*", ".github/nested/check.yml", ".github-other/nested/check.yml"},
		{"/docs/*.md", "docs/notes.md/child.txt", "docs/adr/0001.md"},
	} {
		t.Run(tc.pattern, func(t *testing.T) {
			t.Parallel()
			owners := "* @core\n" + tc.pattern + " @other\n"
			got, err := bootstrap.AllowedPaths([]string{tc.owned, tc.near}, []byte(owners))
			if err != nil || len(got.Unsupported) != 0 || !slices.Equal(got.Protected, []string{tc.owned}) ||
				!slices.Equal(got.Allowed, []string{tc.near}) {
				t.Fatalf("wildcard directory ownership = %+v, %v", got, err)
			}
		})
	}
}

func TestOwnershipComparesNamedOwnerSetsWithoutCase(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, baseline, assigned string }{
		{"user", "@Core", "@core"},
		{"organization", "@Org/team", "@org/team"},
		{"team", "@org/Team", "@org/team"},
		{"normalized set", "@CORE @Org/Team @Core", "@org/team @core"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			owners := "* " + tc.baseline + "\n/golaw.yml " + tc.assigned + "\n/other.yml @core2\n"
			got, err := bootstrap.AllowedPaths([]string{"golaw.yml", "other.yml"}, []byte(owners))
			if err != nil || len(got.Unsupported) != 0 || !slices.Equal(got.Allowed, []string{"golaw.yml"}) ||
				!slices.Equal(got.Protected, []string{"other.yml"}) {
				t.Fatalf("case-normalized owners = %+v, %v", got, err)
			}
		})
	}
}

func TestOwnershipGlobalCatchallFormsSetTheBaseline(t *testing.T) {
	t.Parallel()
	for _, pattern := range []string{"*", "**", "/*", "/**", "**/*"} {
		t.Run(pattern, func(t *testing.T) {
			t.Parallel()
			owners := pattern + " @core\n/owned/ @other\n"
			paths := []string{"golaw.yml", "nested/config.yml", "owned/config.yml"}
			got, err := bootstrap.AllowedPaths(paths, []byte(owners))
			if err != nil || len(got.Unsupported) != 0 ||
				!slices.Equal(got.Allowed, []string{"golaw.yml", "nested/config.yml"}) ||
				!slices.Equal(got.Protected, []string{"owned/config.yml"}) {
				t.Fatalf("global catchall ownership = %+v, %v", got, err)
			}
		})
	}
}

func TestOwnershipFinalGlobalCatchallSetsTheBaseline(t *testing.T) {
	t.Parallel()
	for _, pattern := range []string{"*", "**", "/*", "/**", "**/*"} {
		t.Run(pattern, func(t *testing.T) {
			t.Parallel()
			owners := "* @old\n" + pattern + " @core\n/golaw.yml @core\n/owned/ @old\n"
			got, err := bootstrap.AllowedPaths([]string{"golaw.yml", "owned/config.yml"}, []byte(owners))
			if err != nil || len(got.Unsupported) != 0 || !slices.Equal(got.Allowed, []string{"golaw.yml"}) ||
				!slices.Equal(got.Protected, []string{"owned/config.yml"}) {
				t.Fatalf("last global catchall ownership = %+v, %v", got, err)
			}
		})
	}
}

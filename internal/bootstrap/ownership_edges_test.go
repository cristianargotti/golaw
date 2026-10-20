package bootstrap_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
)

func TestOwnershipNormalizesAssignmentsWithoutInventingOwners(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, owners string
		protected    bool
	}{
		{"last catch all baseline", "* @old\n* @core\n/golaw.yml @old\n", true},
		{"later catch all wins", "* @core\n/golaw.yml @other\n* @next\n", false},
		{"duplicate owners", "* @core @reviewers @core\n/golaw.yml @reviewers @core\n", false},
		{"cleared ownership", "* @core\n/golaw.yml\n", false},
		{"cleared baseline", "* @core\n*\n/golaw.yml @core\n", true},
		{"unmatched without baseline", "/elsewhere @other\n", false},
		{"owned without baseline", "/golaw.yml @other\n", true},
		{"comments and CRLF", " \t# comment\r\n\r\n*\t@core\r\n/golaw.yml @core # @other\r\n", false},
		{"team owners", "* @org/core\n/golaw.yml @org/other\n", true},
		{"named owner near", "* @core\n/golaw.yml @other\n", true},
		{"team owner near", "* @core\n/golaw.yml @org/team\n", true},
		{"team slug characters", "* @core\n/golaw.yml @org-2/team_name-3\n", true},
		{"email owners", "* core@example.test\n/golaw.yml core@example.test\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := bootstrap.AllowedPaths([]string{"golaw.yml"}, []byte(tc.owners))
			if err != nil || len(got.Unsupported) != 0 || slices.Contains(got.Protected, "golaw.yml") != tc.protected {
				t.Fatalf("ownership = %+v, %v", got, err)
			}
			if slices.Contains(got.Allowed, "golaw.yml") == tc.protected {
				t.Fatalf("destination omitted or assigned twice: %+v", got)
			}
		})
	}
}

func TestOwnershipMatchesSupportedComponentsExactly(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		pattern, match, near string
	}{
		{"/docs/**", "docs/file.txt", "docs"},
		{"docs/**", "docs/file.txt", "docs"},
		{"/docs/**/config.yml", "docs/config.yml", "nested/docs/config.yml"},
		{"/docs/**/config.yml", "docs/a/b/config.yml", "docs/a/b/config.yaml"},
		{"**/config.yml", "config.yml", "config.yaml"},
		{"**/config.yml", "a/b/config.yml", "a/b/config.yaml"},
		{"a/**/b/**/config.yml", "a/one/b/two/three/config.yml", "a/one/c/two/config.yml"},
		{"/docs/*/config.yml", "docs/a/config.yml", "docs/a/b/config.yml"},
		{"docs/config.yml", "docs/config.yml", "nested/docs/config.yml"},
		{"/docs/", "docs/a/b.txt", "nested/docs/a/b.txt"},
		{"docs/", "nested/docs/a.txt", "nested/docs-old/a.txt"},
		{"/docs", "docs/a/b.txt", "nested/docs/a/b.txt"},
		{"docs", "nested/docs/a.txt", "nested/docs-old/a.txt"},
		{"doc*/", "nested/docs/a.txt", "nested/manual/a.txt"},
		{"/docs/*/", "docs/team/a.txt", "docs/a.txt"},
		{"config*.yml", "a/b/config-core.yml", "a/b/config-core.yaml"},
		{"a*b*c.txt", "nested/alpha-beta-c.txt", "nested/alpha-beta-c.txt.old"},
		{"a*b*c.txt", "abc.txt", "a/b/c.txt"},
		{"/.github/*", ".github/dependabot.yml", ".github-other/nested/check.yml"},
	} {
		t.Run(tc.pattern+"/"+tc.match, func(t *testing.T) {
			t.Parallel()
			owners := "* @core\n" + tc.pattern + " @other\n"
			got, err := bootstrap.AllowedPaths([]string{tc.match, tc.near}, []byte(owners))
			if err != nil || len(got.Unsupported) != 0 || !slices.Equal(got.Protected, []string{tc.match}) ||
				!slices.Equal(got.Allowed, []string{tc.near}) {
				t.Fatalf("component ownership = %+v, %v", got, err)
			}
		})
	}
}

func TestOwnershipSkipsUnrecognizedOwnerSyntax(t *testing.T) {
	t.Parallel()
	for _, owner := range []string{"team", "@", "other@", "@org/", "@org/team/extra", "@other!", "@org/*"} {
		t.Run(owner, func(t *testing.T) {
			t.Parallel()
			owners := "* @core\n/golaw.yml " + owner + "\n"
			got, err := bootstrap.AllowedPaths([]string{"golaw.yml"}, []byte(owners))
			want := "CODEOWNERS line 2: unsupported owner: /golaw.yml"
			if err != nil || !slices.Equal(got.Unsupported, []string{want}) ||
				!slices.Equal(got.Allowed, []string{"golaw.yml"}) || len(got.Protected) != 0 {
				t.Fatalf("unsupported owner = %+v, %v", got, err)
			}
		})
	}
}

func TestOwnershipUnsupportedLinesKeepSupportedAssignments(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ pattern, reason string }{
		{"!/docs/", "unsupported negation"},
		{"/docs/[ab].md", "unsupported bracket pattern"},
		{"/docs/?.md", "unsupported question wildcard"},
		{`/docs/a\ b.md`, "unsupported escape syntax"},
		{"/docs/**name.md", "unsupported recursive wildcard"},
		{"/docs//name.md", "unsupported path component"},
		{"/docs/../name.md", "unsupported path component"},
	} {
		t.Run(tc.reason, func(t *testing.T) {
			t.Parallel()
			owners := "# heading\n* @core\n" + tc.pattern + " @other\n/protected.txt @other\n"
			got, err := bootstrap.AllowedPaths([]string{"golaw.yml", "protected.txt"}, []byte(owners))
			pattern := strings.Fields(tc.pattern)[0]
			want := "CODEOWNERS line 3: " + tc.reason + ": " + pattern
			if err != nil || !slices.Equal(got.Unsupported, []string{want}) ||
				!slices.Equal(got.Allowed, []string{"golaw.yml"}) ||
				!slices.Equal(got.Protected, []string{"protected.txt"}) {
				t.Fatalf("unsupported ownership = %+v, %v", got, err)
			}
		})
	}
}

func TestOwnershipKeepsDestinationOrderAndInputs(t *testing.T) {
	t.Parallel()
	paths := []string{"z.txt", "protected.txt", "a.txt"}
	before := slices.Clone(paths)
	owners := []byte("* @core\n/protected.txt @other\n")
	got, err := bootstrap.AllowedPaths(paths, owners)
	if err != nil || !slices.Equal(paths, before) || string(owners) != "* @core\n/protected.txt @other\n" ||
		!slices.Equal(got.Allowed, []string{"z.txt", "a.txt"}) || !slices.Equal(got.Protected, []string{"protected.txt"}) {
		t.Fatalf("ordered ownership = %+v, %v", got, err)
	}
}

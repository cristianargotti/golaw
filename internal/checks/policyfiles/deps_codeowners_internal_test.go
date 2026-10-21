package policyfiles

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

func TestOwnersRetainAssignmentsAndBaseline(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, data, path string
		want             []string
		line             int
		foreign          bool
	}{
		{name: "empty", data: "\n # comment\n", path: "docs/a.md"},
		{
			name: "normalization and last assignment",
			data: "* @app\n/docs/ @OTHER\n/docs/a.md @Beta @ALPHA @beta # note\n",
			path: "docs/a.md", want: []string{"@alpha", "@beta"}, line: 3, foreign: true,
		},
		{
			name: "equal sets", data: "* @A @B\n/docs/ @b @a @A\n", path: "docs/a.md",
			want: []string{"@a", "@b"}, line: 2,
		},
		{name: "cleared ownership", data: "* @app\n/docs/ # clear\n", path: "docs/a.md", line: 2},
		{
			name: "root baseline fallback", data: "/* @app\n/docs/ @app\n", path: "docs/a.md",
			want: []string{"@app"}, line: 2,
		},
		{
			name: "empty catchall precedes root fallback", data: "*\n/* @app\n/docs/ @app\n",
			path: "docs/a.md", want: []string{"@app"}, line: 3, foreign: true,
		},
		{
			name: "latest catchall", data: "** @old\n/** @new\n**/* @app\n/docs/ @app\n",
			path: "docs/a.md", want: []string{"@app"}, line: 4,
		},
		{
			name: "team names", data: "* @Org/Team_1\n/docs/ @org/team_1\n", path: "docs/a.md",
			want: []string{"@org/team_1"}, line: 2,
		},
		{
			name: "email case retained", data: "* app@example.test\n/docs/ App@example.test\n",
			path: "docs/a.md", want: []string{"App@example.test"}, line: 2, foreign: true,
		},
		{name: "no assignment", data: "/docs/ @other\n", path: "src/a.go"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := readOwners("CODEOWNERS", []byte(tc.data))
			if err != nil {
				t.Fatal(err)
			}
			assigned := got.assignment(tc.path)
			foreign, line := got.foreign(tc.path)
			if !slices.Equal(assigned.owners, tc.want) || assigned.line != tc.line ||
				foreign != tc.foreign || line != tc.line {
				t.Fatalf("assignment = %+v, foreign = %v, line = %d", assigned, foreign, line)
			}
		})
	}
}

func TestOwnersRejectUnsupportedEvidence(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		assignment, reason string
	}{
		{"!/docs/ @other", "negation"},
		{"/docs/[ab] @other", "bracket"},
		{"/docs/a? @other", "question"},
		{"/docs/a\\b @other", "escape"},
		{"/docs/**a @other", "recursive"},
		{"/docs/*** @other", "recursive"},
		{"/docs//a @other", "component"},
		{"/docs/./a @other", "component"},
		{"/docs/../a @other", "component"},
		{"/ @other", "component"},
		{"/docs/ other", "owner"},
		{"/docs/ @", "owner"},
		{"/docs/ @org/", "owner"},
		{"/docs/ @/team", "owner"},
		{"/docs/ @bad_account", "owner"},
		{"/docs/ @org/team/more", "owner"},
		{"/docs/ app@@example.test", "owner"},
		{"/docs/ app@", "owner"},
		{"/docs/ a/b@example.test", "owner"},
		{"/docs/ app@example/test", "owner"},
	} {
		t.Run(tc.assignment, func(t *testing.T) {
			t.Parallel()
			got, err := readOwners("CODEOWNERS", []byte("* @app\n"+tc.assignment+"\n"))
			if !errors.Is(err, model.ErrOutsideSubset) || !strings.Contains(err.Error(), "CODEOWNERS:2:") ||
				!strings.Contains(err.Error(), tc.reason) || len(got.rules) != 0 {
				t.Fatalf("owners = %+v, error = %v", got, err)
			}
		})
	}
}

func TestOwnerPatternsKeepComponentBoundaries(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		pattern, path string
		match         bool
	}{
		{"/docs/", "docs/a.md", true},
		{"/docs/", "a/docs/a.md", false},
		{"docs/", "a/docs/b.md", true},
		{"docs/", "docs", false},
		{"docs/", "a/docs-old/b.md", false},
		{"/docs/*", "docs/a.md", true},
		{"/docs/*", "docs/a/b.md", false},
		{"/docs/**", "docs/a/b.md", true},
		{"/docs/**", "docs", false},
		{"/docs/**", "manual/a.md", false},
		{"**/docs/**", "a/docs/b/c.md", true},
		{"/docs/**/a.md", "docs/a.md", true},
		{"/docs/**/a.md", "docs/a/b/a.md", true},
		{"*.md", "docs/a.md", true},
		{"*.md", "docs/a.go", false},
		{"a*b*c.md", "a-first-b-last-c.md", true},
		{"a*b*c.md", "z-first-b-last-c.md", false},
		{"a*b*c.md", "a-first-z-last-c.md", false},
		{".Dockerfile", "Dockerfile", false},
		{".Dockerfile", "a/.Dockerfile", true},
		{"/*", "a.md", true},
		{"/*", "docs/a.md", false},
	} {
		t.Run(tc.pattern+" "+tc.path, func(t *testing.T) {
			t.Parallel()
			if got := ownerMatches(tc.pattern, tc.path); got != tc.match {
				t.Fatalf("match = %v, want %v", got, tc.match)
			}
		})
	}
}

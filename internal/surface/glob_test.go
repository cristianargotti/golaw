package surface_test

import (
	"errors"
	"testing"

	"github.com/cristianargotti/golaw/internal/surface"
)

func TestMatchFollowsTheSonarGlob(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		pattern, path string
		want          bool
	}{
		"a test file at depth":                   {pattern: "**/*_test.go", path: "cmd/hub/main_test.go", want: true},
		"a test file at the root":                {pattern: "**/*_test.go", path: "main_test.go", want: true},
		"a main file (near)":                     {pattern: "**/*_test.go", path: "cmd/hub/main.go"},
		"a directory and below":                  {pattern: "integration/**", path: "integration/a/b.go", want: true},
		"a file of the directory":                {pattern: "integration/**", path: "integration/flow.go", want: true},
		"a directory that only starts so (near)": {pattern: "integration/**", path: "integrations/flow.go"},
		"the directory below the root (near)":    {pattern: "integration/**", path: "pkg/integration/flow.go"},
		"one level of mocks":                     {pattern: "test/mocks/*", path: "test/mocks/api.go", want: true},
		"two levels of mocks (near)":             {pattern: "test/mocks/*", path: "test/mocks/sub/api.go"},
		"a gen directory at depth":               {pattern: "**/gen/**", path: "internal/gen/api.go", want: true},
		"a gen directory at the root":            {pattern: "**/gen/**", path: "gen/api.go", want: true},
		"a directory named like gen (near)":      {pattern: "**/gen/**", path: "general/api.go"},
		"a fixture at depth":                     {pattern: "**/testdata/**", path: "internal/panel/testdata/growth.jsx", want: true},
		"one character":                          {pattern: "views/?.js", path: "views/a.js", want: true},
		"two characters (near)":                  {pattern: "views/?.js", path: "views/ab.js"},
		"a trailing slash is the directory":      {pattern: "views/", path: "views/a/b.js", want: true},
		"another case (near)":                    {pattern: "**/*.JS", path: "views/a.js"},
		"everything":                             {pattern: "**", path: "a/b/c.txt", want: true},
		"zero directories in the middle":         {pattern: "src/**/*.ts", path: "src/c.ts", want: true},
		"several directories in the middle":      {pattern: "src/**/*.ts", path: "src/a/b/c.ts", want: true},
		"a star never crosses a slash (near)":    {pattern: "src/*.ts", path: "src/a/c.ts"},
		"a pattern longer than the path (near)":  {pattern: "a/b/c", path: "a/b"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := surface.Match(tc.pattern, tc.path)
			if err != nil || got != tc.want {
				t.Fatalf("Match(%q, %q) = %v, %v, want %v", tc.pattern, tc.path, got, err, tc.want)
			}
		})
	}
}

func TestValidPatternRefusesWhatItWouldGuess(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"empty":                          "",
		"braces":                         "**/*.test.{js,ts}",
		"a class":                        "views/[ab].js",
		"an escape":                      `views\a.js`,
		"absolute":                       "/srv/app/**",
		"an empty segment":               "a//b",
		"a dot segment":                  "./a",
		"a parent segment":               "../a",
		"two stars inside a segment":     "a/**b",
		"two stars around a name":        "**name**",
		"a lone slash":                   "/",
		"a trailing double slash":        "a//",
		"a dot directory with a slash":   "./",
		"stars that are not one segment": "a/***",
	}
	for name, pattern := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := surface.ValidPattern(pattern); !errors.Is(err, surface.ErrPattern) {
				t.Fatalf("ValidPattern(%q) = %v, want ErrPattern", pattern, err)
			}
			if got, err := surface.Match(pattern, "a/b"); got || !errors.Is(err, surface.ErrPattern) {
				t.Fatalf("Match(%q) = %v, %v, want false and ErrPattern", pattern, got, err)
			}
		})
	}
}

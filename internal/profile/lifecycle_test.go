package profile_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/profile"
	"github.com/cristianargotti/golaw/internal/profiletest"
)

func TestParseSourceSplitsTheURLAndTheTag(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		arg  string
		want profile.Source
	}{
		"https with v":    {"https://example.test/profiles/fixture@v1.2.3", profile.Source{URL: "https://example.test/profiles/fixture", Tag: "v1.2.3", Version: "1.2.3"}},
		"https without v": {"https://example.test/profiles/fixture@1.2.3", profile.Source{URL: "https://example.test/profiles/fixture", Tag: "1.2.3", Version: "1.2.3"}},
		"ssh with user":   {"git@example.test:org/fixture.git@v0.4.0", profile.Source{URL: "git@example.test:org/fixture.git", Tag: "v0.4.0", Version: "0.4.0"}},
		"a local path":    {"/srv/profiles/fixture.git@v10.20.30", profile.Source{URL: "/srv/profiles/fixture.git", Tag: "v10.20.30", Version: "10.20.30"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := profile.ParseSource(tc.arg)
			if err != nil || got != tc.want {
				t.Fatalf("ParseSource(%q) = %+v, %v, want %+v", tc.arg, got, err, tc.want)
			}
		})
	}
}

func TestParseSourceRefusesWhatIsNotASource(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		arg  string
		want string
	}{
		"no tag":        {"https://example.test/profiles/fixture", "<git url>@<tag>"},
		"empty tag":     {"https://example.test/profiles/fixture@", "<git url>@<tag>"},
		"empty url":     {"@v1.2.3", "<git url>@<tag>"},
		"ssh, no tag":   {"git@example.test:org/fixture.git", `"git@example.test:org/fixture.git" is not <git url>@<tag>`},
		"not a version": {"https://example.test/profiles/fixture@latest", `tag "latest" is not vX.Y.Z or X.Y.Z`},
		"a range":       {"https://example.test/profiles/fixture@v1.2", `tag "v1.2" is not vX.Y.Z or X.Y.Z`},
		"a suffix":      {"https://example.test/profiles/fixture@v1.2.3-rc1", `tag "v1.2.3-rc1" is not vX.Y.Z or X.Y.Z`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := profile.ParseSource(tc.arg)
			if !errors.Is(err, model.ErrUsage) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ParseSource(%q) = %v, want %q", tc.arg, err, tc.want)
			}
		})
	}
}

func TestLifecycleAnswersNotBuiltUntilItsPart(t *testing.T) {
	t.Parallel()
	dirs := policy.Dirs{State: t.TempDir(), Config: t.TempDir()}
	if err := profile.Remove(dirs, profiletest.Name); !errors.Is(err, profile.ErrNotBuilt) {
		t.Fatalf("Remove = %v", err)
	}
}

// now is the fixed clock of the lifecycle tests.
func now() time.Time { return time.Date(2026, time.October, 21, 12, 0, 0, 0, time.UTC) }

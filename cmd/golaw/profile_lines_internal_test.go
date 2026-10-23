package main

import (
	"slices"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/profile"
)

// installedFixture is an installed version as the record keeps it, from a
// mirror of its canonical source.
func installedFixture() profile.Installed {
	return profile.Installed{
		Name: "fixture", Version: "1.2.3", Digest: "sha256:aa", Source: "https://example.test/profiles/fixture",
		URL: "/srv/fixture.git", Tag: "v1.2.3", InstalledAt: time.Date(2026, time.October, 21, 12, 0, 0, 0, time.UTC),
	}
}

func TestProfileLinesSpeakPortuguese(t *testing.T) {
	t.Parallel()
	installed := installedFixture()
	pin := "golaw.yml não mudou: para fixar fixture 1.2.3 em um repositório, execute golaw init --profile fixture --apply e faça commit de golaw.yml"
	cases := map[string]struct{ got, want string }{
		"installed":   {installedLine(installed, false), "Perfil fixture 1.2.3 instalado de /srv/fixture.git@v1.2.3, digest sha256:aa"},
		"already":     {installedLine(installed, true), "Perfil fixture 1.2.3 já estava instalado"},
		"rolled back": {rolledBackLine(installed), "Perfil fixture voltou para 1.2.3"},
		"removed":     {removedLine("fixture"), "Perfil fixture removido"},
		"pin":         {pinHintLine(installed), pin},
	}
	for name, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s: %q, want %q", name, tc.got, tc.want)
		}
	}
}

func TestProfileLinesComposeEachStep(t *testing.T) {
	t.Parallel()
	mirror := installedFixture()
	canonical := mirror
	canonical.URL = canonical.Source
	source := "  source: https://example.test/profiles/fixture (the source golaw.yml pins)"
	cases := map[string]struct{ got, want []string }{
		"an install from a mirror":   {installLines(mirror, false), []string{installedLine(mirror, false), source, pinHintLine(mirror)}},
		"an install from the source": {installLines(canonical, false), []string{installedLine(canonical, false), pinHintLine(canonical)}},
		"an install already done":    {installLines(mirror, true), []string{installedLine(mirror, true)}},
		"a refresh from a mirror":    {refreshLines(mirror), []string{installedLine(mirror, false), source}},
		"a refresh from the source":  {refreshLines(canonical), []string{installedLine(canonical, false)}},
		"a rollback names the pin":   {rolledBackLines(mirror), []string{rolledBackLine(mirror), pinHintLine(mirror)}},
	}
	for name, tc := range cases {
		if !slices.Equal(tc.got, tc.want) {
			t.Errorf("%s: %q, want %q", name, tc.got, tc.want)
		}
	}
}

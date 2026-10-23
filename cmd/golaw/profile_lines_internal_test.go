package main

import (
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/profile"
)

func TestProfileLinesSpeakPortuguese(t *testing.T) {
	t.Parallel()
	installed := profile.Installed{
		Name: "fixture", Version: "1.2.3", Digest: "sha256:aa", Source: "https://example.test/profiles/fixture",
		URL: "/srv/fixture.git", Tag: "v1.2.3", InstalledAt: time.Date(2026, time.October, 21, 12, 0, 0, 0, time.UTC),
	}
	cases := map[string]struct{ got, want string }{
		"installed":   {installedLine(installed, false), "Perfil fixture 1.2.3 instalado de /srv/fixture.git@v1.2.3, digest sha256:aa"},
		"already":     {installedLine(installed, true), "Perfil fixture 1.2.3 já estava instalado"},
		"rolled back": {rolledBackLine(installed), "Perfil fixture voltou para 1.2.3"},
		"removed":     {removedLine("fixture"), "Perfil fixture removido"},
	}
	for name, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s: %q, want %q", name, tc.got, tc.want)
		}
	}
}

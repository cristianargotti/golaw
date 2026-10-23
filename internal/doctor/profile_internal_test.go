package doctor

import "testing"

func TestAssessProfileReportsThePinBesideTheBundle(t *testing.T) {
	t.Parallel()
	missing := "profile fixture is not installed: run golaw profile install https://example.test/profiles/fixture@1.2.3"
	installed := ProfileFacts{
		Name: "fixture", PinnedVersion: "1.2.3", PinnedDigest: "sha256:aa", Source: "https://example.test/profiles/fixture",
		InstalledVersion: "1.2.3", InstalledDigest: "sha256:aa", Date: "2026-10-20", Age: 2,
	}
	broken := installed
	broken.InstalledVersion, broken.InstalledDigest, broken.Date, broken.Age, broken.Error = "", "", "", 0, missing
	cases := map[string]struct {
		facts ProfileFacts
		want  Line
	}{
		"core only": {ProfileFacts{}, Line{
			ID: "doctor.profile", Key: "none", State: "ok", Code: "profile-none",
			Detail: "golaw.yml names no profile: the core rules alone are in force",
		}},
		"installed": {installed, Line{
			ID: "doctor.profile", Key: "fixture", State: "ok", Code: "profile-installed",
			Detail: "version 1.2.3, digest sha256:aa, 2 days since 2026-10-20",
		}},
		"not installed": {broken, Line{
			ID: "doctor.profile", Key: "fixture", State: "red", Code: "profile-not-installed", Detail: missing,
		}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := AssessProfile(tc.facts); got != tc.want {
				t.Fatalf("AssessProfile(%+v) = %+v, want %+v", tc.facts, got, tc.want)
			}
		})
	}
}

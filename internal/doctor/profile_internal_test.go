package doctor

import "testing"

// installedProfileFacts are the facts of the fixture bundle loaded at its pin.
func installedProfileFacts() ProfileFacts {
	return ProfileFacts{
		Name: "fixture", PinnedVersion: "1.2.3", PinnedDigest: "sha256:aa", Source: "https://example.test/profiles/fixture",
		InstalledVersion: "1.2.3", InstalledDigest: "sha256:aa", ComputedDigest: "sha256:aa", Date: "2026-10-20", Age: 2,
	}
}

// refused are the installed facts with a load error and one change.
func refused(change func(*ProfileFacts)) ProfileFacts {
	facts := installedProfileFacts()
	facts.Error = "the loader's own words: run golaw profile install https://example.test/profiles/fixture@1.2.3"
	change(&facts)
	return facts
}

func TestAssessProfileNamesTheFaultTheFactsShow(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		facts ProfileFacts
		state string
		code  string
	}{
		"core only":                   {ProfileFacts{}, "ok", "profile-none"},
		"installed":                   {installedProfileFacts(), "ok", "profile-installed"},
		"not installed":               {refused(func(f *ProfileFacts) { f.InstalledVersion, f.InstalledDigest, f.ComputedDigest = "", "", "" }), "red", "profile-not-installed"},
		"behind the pin":              {refused(func(f *ProfileFacts) { f.InstalledVersion = "1.1.0" }), "red", "profile-stale"},
		"behind by a two digit minor": {refused(func(f *ProfileFacts) { f.PinnedVersion, f.InstalledVersion = "1.10.0", "1.9.0" }), "red", "profile-stale"},
		"ahead of the pin":            {refused(func(f *ProfileFacts) { f.InstalledVersion = "1.3.0" }), "red", "profile-version-mismatch"},
		"another digest in the stamp": {refused(func(f *ProfileFacts) { f.InstalledDigest, f.ComputedDigest = "sha256:bb", "sha256:bb" }), "red", "profile-digest-mismatch"},
		"files changed under it":      {refused(func(f *ProfileFacts) { f.ComputedDigest = "sha256:cc" }), "red", "profile-digest-mismatch"},
		"files that would not hash":   {refused(func(f *ProfileFacts) { f.ComputedDigest = "" }), "red", "profile-data-invalid"},
		"data the loader refuses":     {refused(func(*ProfileFacts) {}), "red", "profile-data-invalid"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			line := AssessProfile(tc.facts)
			if line.ID != "doctor.profile" || line.State != tc.state || line.Code != tc.code {
				t.Fatalf("AssessProfile(%+v) = %+v, want %s %s", tc.facts, line, tc.state, tc.code)
			}
			if tc.state == "red" && line.Detail != tc.facts.Error {
				t.Fatalf("a red line carries the loader's words: %q", line.Detail)
			}
		})
	}
}

func TestAssessProfileDetailsTheInstalledBundle(t *testing.T) {
	t.Parallel()
	installed := AssessProfile(installedProfileFacts())
	if installed.Key != "fixture" || installed.Detail != "version 1.2.3, digest sha256:aa, 2 days since 2026-10-20" {
		t.Fatalf("installed = %+v", installed)
	}
	none := AssessProfile(ProfileFacts{})
	if none.Key != "none" || none.Detail != "golaw.yml names no profile: the core rules alone are in force" {
		t.Fatalf("core only = %+v", none)
	}
}

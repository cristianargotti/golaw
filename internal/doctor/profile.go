package doctor

import (
	"fmt"

	"golang.org/x/mod/semver"
)

// The line of the pinned profile bundle (section 5.6, A19).
const (
	idProfile                  = "doctor.profile"
	codeProfileNone            = "profile-none"
	codeProfileInstalled       = "profile-installed"
	codeProfileNotInstalled    = "profile-not-installed"
	codeProfileVersionMismatch = "profile-version-mismatch"
	codeProfileDigestMismatch  = "profile-digest-mismatch"
	codeProfileStale           = "profile-stale"
	codeProfileDataInvalid     = "profile-data-invalid"
)

// ProfileFacts is the evidence of the profile golaw.yml pins beside the
// bundle found for it under the config directory, collected by cmd so the
// assessment reads no file.
type ProfileFacts struct {
	Name             string `json:"name"`              // from golaw.yml; empty for core only
	PinnedVersion    string `json:"pinned_version"`    // from golaw.yml
	PinnedDigest     string `json:"pinned_digest"`     // from golaw.yml
	Source           string `json:"source"`            // from golaw.yml
	InstalledVersion string `json:"installed_version"` // from the stamp found for the pin; empty when none of its source was
	InstalledDigest  string `json:"installed_digest"`  // from the stamp found for the pin
	ComputedDigest   string `json:"computed_digest"`   // recomputed over the files of that stamp's bundle; empty when none was
	Date             string `json:"date"`              // the date stamp of the bundle
	Age              int    `json:"age"`               // days from Date to the clock
	Error            string `json:"error"`             // the load error, which names the install command; empty when the bundle loaded
}

// AssessProfile is the profile line: core only is a fact, a bundle that
// loaded names its version, digest and age, and a bundle the load refused
// is red with the load's own words, which name the exact command (A19),
// under the code of the fault the facts show.
func AssessProfile(facts ProfileFacts) Line {
	switch {
	case facts.Name == "":
		return Line{
			ID: idProfile, Key: "none", State: stateOK, Code: codeProfileNone,
			Detail: "golaw.yml names no profile: the core rules alone are in force",
		}
	case facts.Error != "":
		return Line{ID: idProfile, Key: facts.Name, State: stateRed, Code: profileFault(facts), Detail: facts.Error}
	}
	return Line{
		ID: idProfile, Key: facts.Name, State: stateOK, Code: codeProfileInstalled,
		Detail: fmt.Sprintf("version %s, digest %s, %d days since %s", facts.InstalledVersion, facts.InstalledDigest, facts.Age, facts.Date),
	}
}

// profileFault names why the pinned bundle did not load, in the order a
// person fixes it: no bundle of the pinned source, a bundle behind the pin
// or at another version, other files than the pin or the stamp names,
// and last the data itself.
func profileFault(facts ProfileFacts) string {
	switch {
	case facts.InstalledVersion == "":
		return codeProfileNotInstalled
	case facts.InstalledVersion != facts.PinnedVersion && semver.Compare("v"+facts.InstalledVersion, "v"+facts.PinnedVersion) < 0:
		return codeProfileStale
	case facts.InstalledVersion != facts.PinnedVersion:
		return codeProfileVersionMismatch
	case facts.InstalledDigest != facts.PinnedDigest, facts.ComputedDigest != "" && facts.ComputedDigest != facts.InstalledDigest:
		return codeProfileDigestMismatch
	}
	return codeProfileDataInvalid
}

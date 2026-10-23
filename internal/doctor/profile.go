package doctor

import "fmt"

// The line of the pinned profile bundle (section 5.6, A19).
const (
	idProfile               = "doctor.profile"
	codeProfileNone         = "profile-none"
	codeProfileInstalled    = "profile-installed"
	codeProfileNotInstalled = "profile-not-installed"
)

// ProfileFacts is the evidence of the profile golaw.yml pins beside the
// bundle the load found under the config directory, collected by cmd so
// the assessment reads no file.
type ProfileFacts struct {
	Name             string `json:"name"`              // from golaw.yml; empty for core only
	PinnedVersion    string `json:"pinned_version"`    // from golaw.yml
	PinnedDigest     string `json:"pinned_digest"`     // from golaw.yml
	Source           string `json:"source"`            // from golaw.yml
	InstalledVersion string `json:"installed_version"` // from the stamp the load found
	InstalledDigest  string `json:"installed_digest"`  // from the stamp the load found
	Date             string `json:"date"`              // the date stamp of the bundle
	Age              int    `json:"age"`               // days from Date to the clock
	Error            string `json:"error"`             // the load error, which names the install command; empty when the bundle loaded
}

// AssessProfile is the profile line: core only is a fact, a bundle that
// loaded names its version, digest and age, and a bundle the load refused
// is red with the load's own words, which name the exact command (A19).
func AssessProfile(facts ProfileFacts) Line {
	switch {
	case facts.Name == "":
		return Line{
			ID: idProfile, Key: "none", State: stateOK, Code: codeProfileNone,
			Detail: "golaw.yml names no profile: the core rules alone are in force",
		}
	case facts.Error != "":
		return Line{ID: idProfile, Key: facts.Name, State: stateRed, Code: codeProfileNotInstalled, Detail: facts.Error}
	}
	return Line{
		ID: idProfile, Key: facts.Name, State: stateOK, Code: codeProfileInstalled,
		Detail: fmt.Sprintf("version %s, digest %s, %d days since %s", facts.InstalledVersion, facts.InstalledDigest, facts.Age, facts.Date),
	}
}

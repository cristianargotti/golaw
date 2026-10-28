package model

// The sections of a profile bundle's data; Section answers them by name.
const (
	SectionAssertions = "assertions"
	SectionDockerfile = "dockerfile"
	SectionLayout     = "layout"
	SectionPaths      = "paths"
	SectionRegistry   = "registry"
	SectionTargets    = "targets"
	SectionTemplates  = "templates"
	SectionTerms      = "terms"
	SectionAnalyzer   = "analyzer"
	SectionSonarWay   = "sonar-way"
)

// The kinds a typed YAML assertion compares.
const (
	AssertBool   = "bool"
	AssertInt    = "int"
	AssertString = "string"
)

// The explicit rules of an assertion for a missing key and a missing file.
const (
	MissingFinding = "finding"
	MissingIgnore  = "ignore"
	FilesRequired  = "required"
	FilesOptional  = "optional"
)

// ProfileData is the data of the enrolled profile bundle as the loader
// validated it (section 5.4): every section optional, and each one only
// adds a check or a value (A1); there is no key that names a floor.
type ProfileData struct {
	Name       string           // the profile name, equal to EffectivePolicy.Profile
	Version    string           // the exact version of the bundle
	Digest     string           // sha256:<hex> over the bundle files, what golaw.yml pins
	Terms      []string         // extra forbidden terms, lower case, merged into TermPolicy.Tools
	Paths      []string         // forbidden path globs in slash form, judged by paths.profile-forbidden
	Layout     *Layout          // the layout init renders under the profile when the repository names none
	Templates  TemplateRules    // what init leaves out under the profile (A1)
	Targets    []string         // make targets the ci target must carry beyond the core table
	Dockerfile *DockerfileRules // the image rules, judged under the image capability
	Registry   *RegistryRule    // the image reference pattern, judged under the image capability
	Assertions []Assertion      // typed YAML assertions
	Analyzer   *AnalyzerPolicy  // the bundle's analyzer.yml, nil when it carries none
	SonarWay   *SonarWay        // the bundle's sonar-way/ snapshot, nil when it carries none
}

// TemplateRules says what init leaves to another owner under the profile.
type TemplateRules struct {
	Exclude []string // destination prefixes in slash form; a trailing slash covers a tree, otherwise one file
}

// DockerfileRules are the image rules a profile adds.
type DockerfileRules struct {
	Stages         []string           // the named stages in the order they must appear, each once
	FinalImage     string             // the image of the last stage, such as scratch; empty leaves it free
	Installs       []InstallAllowlist // per stage, the packages an install line may name; a stage absent is free
	PinnedInstalls bool               // every package of an install line carries =version
	IgnoreComments []string           // comment prefixes a Dockerfile may not carry
}

// InstallAllowlist names the packages one stage may install.
type InstallAllowlist struct {
	Stage    string
	Packages []string
}

// RegistryRule is the pattern every FROM reference must match.
type RegistryRule struct {
	Pattern string // RE2; a stage name and scratch are never judged
}

// Assertion is one typed YAML assertion with its explicit missing rules.
type Assertion struct {
	Path    string   // one file, or a glob in the path.Match grammar, slash form
	Key     []string // the key path into the document, one segment per element
	Kind    string   // AssertBool, AssertInt or AssertString
	Equals  *string  // the expected scalar as text; nil asserts presence alone
	Missing string   // MissingFinding or MissingIgnore
	Files   string   // FilesRequired or FilesOptional
}

// Section reports whether the data carries the named section; false for a
// nil receiver and for an unknown name, so a check gates on one call.
func (d *ProfileData) Section(name string) bool {
	if d == nil {
		return false
	}
	switch name {
	case SectionTerms:
		return len(d.Terms) > 0
	case SectionPaths:
		return len(d.Paths) > 0
	case SectionLayout:
		return d.Layout != nil
	case SectionTemplates:
		return len(d.Templates.Exclude) > 0
	case SectionTargets:
		return len(d.Targets) > 0
	case SectionDockerfile:
		return d.Dockerfile != nil
	case SectionRegistry:
		return d.Registry != nil
	case SectionAssertions:
		return len(d.Assertions) > 0
	case SectionAnalyzer:
		return d.Analyzer != nil
	case SectionSonarWay:
		return d.SonarWay != nil
	default:
		return false
	}
}

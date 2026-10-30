package model

// The two rule sets of surface.rules (M6.4.1): parity is the core floor a
// policy that names none holds, strict is what init --apply writes, and a
// change only tightens from parity to strict.
const (
	SurfaceParity = "parity"
	SurfaceStrict = "strict"
)

// The sources surface.analyzer.from names (M6.3.2): auto reads the profile,
// then sonar-project.properties, then golaw.yml; the others pin one source.
const (
	FromAuto       = "auto"
	FromProfile    = "profile"
	FromProperties = "properties"
	FromPolicy     = "policy"
)

// DefaultCoverageReport is the report the pipeline imports when no source
// names another.
const DefaultCoverageReport = "profile.cov"

// DefaultCoverageNew is the Sonar way threshold of coverage on new code, the
// floor of checks.coverage.new_code.
const DefaultCoverageNew = 80

// defaultCoverMode is the mode go test writes without -covermode.
const defaultCoverMode = "set"

// CoverProfile is how the pipeline's test step writes its report.
type CoverProfile struct {
	CoverPkg  bool   // -coverpkg is passed
	CoverMode string // "set" unless the profile names another
}

// AnalyzerPolicy is the analyzer scope a policy or a profile describes
// (M6.3.2, M6.3.3); every list holds Sonar glob patterns in slash form.
type AnalyzerPolicy struct {
	From               string // FromAuto unless golaw.yml names another; empty in a profile
	Sources            []string
	Tests              []string
	TestInclusions     []string
	Exclusions         []string
	CoverageExclusions []string
	CoverageReport     string       // DefaultCoverageReport unless named
	Coverage           CoverProfile // only a profile's analyzer.yml sets it
}

// SurfacePolicy is the surface block of golaw.yml as compiled.
type SurfacePolicy struct {
	Rules    string         // SurfaceParity or SurfaceStrict; a floor
	Analyzer AnalyzerPolicy // a description of the pipeline, outside the floors and the digest
	Globals  []string       // surface.scripts.globals, sorted and unique
}

// DefaultSurface is the surface block of a policy that names none: the
// parity floor, the scope read automatically and the pipeline's report.
func DefaultSurface() SurfacePolicy {
	return SurfacePolicy{
		Rules: SurfaceParity,
		Analyzer: AnalyzerPolicy{
			From:           FromAuto,
			CoverageReport: DefaultCoverageReport,
			Coverage:       CoverProfile{CoverMode: defaultCoverMode},
		},
		Globals: []string{},
	}
}

// SonarWayRule is one active rule of a company server's Sonar way profile,
// as a profile bundle's sonar-way/ snapshot records it.
type SonarWayRule struct {
	Language string            // the Sonar language key: js, ts, css, py, docker, cloudformation, web
	Key      string            // the rule key without the language: S2681
	Severity string            // as the server reports it
	Params   map[string]string // the active parameters
}

// SonarWay is a dated snapshot of a server's activation.
type SonarWay struct {
	Read  string // YYYY-MM-DD, the day the snapshot was read
	Rules []SonarWayRule
}

// Based is a snapshot that folds a hook overlay over a base and reads the
// base apart, so a check judges what a candidate introduces.
type Based interface {
	Base() Snapshot
}

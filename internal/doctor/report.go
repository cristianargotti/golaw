package doctor

import (
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/model"
)

// Line preserves evidence states separately from human presentation.
type Line struct {
	ID     string `json:"id"`
	Key    string `json:"key,omitempty"`
	State  string `json:"state"`
	Code   string `json:"code"`
	Detail string `json:"detail"`
}

// Report makes the JSON surface versioned and deterministic.
// Summary is the sentence the person reads under the lines in the text
// output, in the configured language; the JSON never carries it.
type Report struct {
	Schema  int    `json:"schema"`
	Lines   []Line `json:"lines"`
	Summary string `json:"-"`
}

// Snapshot retains collected inputs so each diagnosis can be reproduced.
type Snapshot struct {
	Receipt            install.Receipt   `json:"receipt"`
	Files              map[string]string `json:"files"`
	Settings           string            `json:"settings"`
	Hooks              string            `json:"hooks"`
	Plugin             string            `json:"plugin"`
	RequiredRead       []string          `json:"required_read"`
	Binary             BinaryEvidence    `json:"binary"`
	ClaudeVersion      string            `json:"claude_version"`
	ClaudeVersionError string            `json:"claude_version_error"`
	PinnedVersion      string            `json:"pinned_version"`
	Build              BuildMetadata     `json:"build"`
	Enrollment         model.Enrollment  `json:"enrollment"`
	EnrollmentError    string            `json:"enrollment_error"`
	Dispatch           Dispatch          `json:"dispatch"`
}

type assessment struct {
	id  string
	run func(Snapshot) []Line
}

func assessments() []assessment {
	return []assessment{
		{idArtifacts, artifactLines},
		{"doctor.binary", binaryLines},
		{"doctor.plugin", pluginLines},
		{"doctor.read", readLines},
		{idHooks, hookLines},
		{"doctor.version", versionLines},
		{"doctor.pin", pinLines},
		{"doctor.build", buildLines},
		{"doctor.enrollment", enrollmentLines},
		{idPython, pythonLines},
		{"doctor.trust", trustLines},
		{idDispatch, dispatchLines},
	}
}

// Assess keeps collection failures from suppressing independent diagnostics.
func Assess(snapshot Snapshot) Report {
	report := Report{Schema: 1, Lines: []Line{}}
	for _, item := range assessments() {
		report.Lines = append(report.Lines, item.run(snapshot)...)
	}
	return report
}

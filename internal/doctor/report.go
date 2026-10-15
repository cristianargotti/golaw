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
type Report struct {
	Schema int    `json:"schema"`
	Lines  []Line `json:"lines"`
}

// Snapshot retains collected inputs so each diagnosis can be reproduced.
type Snapshot struct {
	Receipt         install.Receipt   `json:"receipt"`
	Files           map[string]string `json:"files"`
	Settings        string            `json:"settings"`
	Hooks           string            `json:"hooks"`
	Plugin          string            `json:"plugin"`
	RequiredRead    []string          `json:"required_read"`
	Binary          BinaryEvidence    `json:"binary"`
	ClaudeVersion   string            `json:"claude_version"`
	PinnedVersion   string            `json:"pinned_version"`
	Build           BuildMetadata     `json:"build"`
	Enrollment      model.Enrollment  `json:"enrollment"`
	EnrollmentError string            `json:"enrollment_error"`
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
		{"doctor.trust", trustLines},
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

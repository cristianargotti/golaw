package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/gen"
	"github.com/cristianargotti/golaw/internal/gen/apply"
	"github.com/cristianargotti/golaw/internal/gen/holes"
	"github.com/cristianargotti/golaw/internal/model"
)

// newTitle is what every report of one golaw new run is headed by.
type newTitle struct {
	kind, name string
	asJSON     bool
}

// newJSON is the JSON report of golaw new, the same facts as the lines.
type newJSON struct {
	Kind    string     `json:"kind"`
	Name    string     `json:"name"`
	Written []string   `json:"written"`
	Skipped []skipJSON `json:"skipped"`
	Holes   []holeJSON `json:"holes"`
}

// holeJSON is one open hole as the JSON reports of the engine print it.
type holeJSON struct {
	Path       string `json:"path"`
	Line       int    `json:"line"`
	ID         string `json:"id"`
	Constraint string `json:"constraint"`
}

// skipJSON is one write the engine left undone and why.
type skipJSON struct {
	Path  string `json:"path"`
	Cause string `json:"cause"`
}

// newLines are the English lines of golaw new on stdout.
func newLines(kind, name string, report apply.Report, open []holes.Hole) []string {
	lines := make([]string, 0, 1+len(report.Written)+len(report.Skipped)+len(open))
	lines = append(lines, fmt.Sprintf("golaw new %s %s: %d files written, %d holes open", kind, name, len(report.Written), len(open)))
	for _, rel := range report.Written {
		lines = append(lines, "written "+rel)
	}
	for _, skip := range report.Skipped {
		lines = append(lines, "skipped "+skip.Path+": "+skip.Cause)
	}
	for _, h := range open {
		lines = append(lines, holeLine(h))
	}
	return lines
}

// newPersonLines are the person's pt-BR lines; the constraint of a hole
// stays in English because the agent fills it and reads it as written.
func newPersonLines(kind, name string, report apply.Report, open []holes.Hole) []string {
	lines := make([]string, 0, 1+len(report.Written)+len(open))
	lines = append(lines, fmt.Sprintf("golaw new %s %s: %d arquivos gerados, %d lacunas abertas", kind, name, len(report.Written), len(open)))
	for _, rel := range report.Written {
		lines = append(lines, "arquivo gerado "+rel)
	}
	for _, h := range open {
		lines = append(lines, holePersonLine(h))
	}
	return lines
}

// holeLine is one open hole for the agent.
func holeLine(h holes.Hole) string {
	return strings.TrimSuffix(fmt.Sprintf("hole open %s:%d %s %s", h.Path, h.Line, h.ID, h.Constraint), " ")
}

// holePersonLine is one open hole for the person.
func holePersonLine(h holes.Hole) string {
	return strings.TrimSuffix(fmt.Sprintf("lacuna aberta %s:%d %s %s", h.Path, h.Line, h.ID, h.Constraint), " ")
}

// personal reports whether the person's lines are printed: a text report
// in pt-BR. Any other language reads the English lines already on stdout.
func personal(p *model.EffectivePolicy, asJSON bool) bool {
	return !asJSON && p != nil && p.Language == model.DefaultLanguage
}

// dryRunLines are the complete diff of every write, headed by its path;
// the diff header of the policy file is replaced by that path.
func dryRunLines(kind, name string, writes []gen.Write, s model.Snapshot, open int) []string {
	lines := []string{fmt.Sprintf("golaw new %s %s (dry run): %d files would be written, %d holes would open", kind, name, len(writes), open)}
	for _, w := range writes {
		old, err := s.Read(w.Path)
		if err != nil {
			old = nil
		}
		lines = append(lines, "--- "+w.Path)
		diff := strings.Split(strings.TrimSuffix(bootstrap.Diff(old, w.Content), "\n"), "\n")
		if len(diff) > 2 {
			lines = append(lines, diff[2:]...)
		}
	}
	return lines
}

// newDocument encodes the JSON report of one run.
func newDocument(title newTitle, report apply.Report, open []holes.Hole) (string, error) {
	doc := newJSON{Kind: title.kind, Name: title.name, Written: report.Written, Skipped: []skipJSON{}, Holes: []holeJSON{}}
	if doc.Written == nil {
		doc.Written = []string{}
	}
	for _, skip := range report.Skipped {
		doc.Skipped = append(doc.Skipped, skipJSON(skip))
	}
	for _, h := range open {
		doc.Holes = append(doc.Holes, holeJSON{Path: h.Path, Line: h.Line, ID: h.ID, Constraint: h.Constraint})
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode the report: %w", err)
	}
	return string(data), nil
}

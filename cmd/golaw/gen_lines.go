package main

import (
	"fmt"

	"github.com/cristianargotti/golaw/internal/gen"
	"github.com/cristianargotti/golaw/internal/gen/apply"
	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/holes"
)

// The prefix of every line of golaw gen --check, so the generate-check rung
// and the pipeline log read one shape.
const checkPrefix = "golaw gen --check: "

// genJSON is the JSON report of golaw gen, the same facts as the lines.
type genJSON struct {
	Written   []string       `json:"written"`
	Skipped   []skipJSON     `json:"skipped"`
	Opened    []holeJSON     `json:"opened"`
	Closed    []holeJSON     `json:"closed"`
	Removed   []removedJSON  `json:"removed"`
	Decisions []decisionJSON `json:"decisions"`
	Notices   []string       `json:"notices"`
}

// removedJSON is one entry the run removed, so the agent reads what went.
type removedJSON struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

// decisionJSON is one decision the agent takes, at its file and line.
type decisionJSON struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

// checkJSON is the JSON report of golaw gen --check.
type checkJSON struct {
	Drift      []driftJSON `json:"drift"`
	Holes      []holeJSON  `json:"holes"`
	Unmeasured []string    `json:"unmeasured"`
}

// driftJSON is one drift with the command that fixes it.
type driftJSON struct {
	Path  string `json:"path"`
	Cause string `json:"cause"`
	Fix   string `json:"fix"`
}

// genLines are the English lines of golaw gen on stdout.
func genLines(res gen.Result, report apply.Report, notices []string) []string {
	lines := []string{fmt.Sprintf("golaw gen: %d files changed, %d holes opened, %d closed, %d decisions",
		len(report.Written), len(res.Opened), len(res.Closed), len(res.Decisions))}
	for _, rel := range report.Written {
		lines = append(lines, "written "+rel)
	}
	for _, skip := range report.Skipped {
		lines = append(lines, "skipped "+skip.Path+": "+skip.Cause)
	}
	for i := range res.Removed {
		lines = append(lines, "removed "+res.Removed[i].Kind+" "+res.Removed[i].Name)
	}
	for _, h := range res.Opened {
		lines = append(lines, holeLine(h))
	}
	for _, h := range res.Closed {
		lines = append(lines, "hole closed "+h.Path+" "+h.ID)
	}
	for _, d := range res.Decisions {
		lines = append(lines, fmt.Sprintf("decision %s:%d %s", d.Path, d.Line, d.Text))
	}
	return append(lines, notices...)
}

// genPersonLines are the person's pt-BR lines of golaw gen.
func genPersonLines(res gen.Result, report apply.Report) []string {
	lines := make([]string, 0, 1+len(report.Written)+len(res.Removed)+len(res.Opened)+len(res.Closed)+len(res.Decisions))
	lines = append(lines, fmt.Sprintf("golaw gen: %d arquivos alterados, %d lacunas abertas, %d fechadas, %d decisões",
		len(report.Written), len(res.Opened), len(res.Closed), len(res.Decisions)))
	for _, rel := range report.Written {
		lines = append(lines, "arquivo gerado "+rel)
	}
	for i := range res.Removed {
		lines = append(lines, "entrada removida "+res.Removed[i].Kind+" "+res.Removed[i].Name)
	}
	for _, h := range res.Opened {
		lines = append(lines, holePersonLine(h))
	}
	for _, h := range res.Closed {
		lines = append(lines, "lacuna fechada "+h.Path+" "+h.ID)
	}
	for _, d := range res.Decisions {
		lines = append(lines, fmt.Sprintf("decisão %s:%d %s", d.Path, d.Line, d.Text))
	}
	return lines
}

// checkLines are the English lines of golaw gen --check: what could not be
// measured first, because it decides the exit code, then each finding with
// its fix, then the summary.
func checkLines(drift []gen.Drift, open []holes.Hole, unmeasured []string) []string {
	lines := make([]string, 0, len(drift)+len(open)+len(unmeasured)+1)
	for _, entry := range unmeasured {
		lines = append(lines, checkPrefix+"unmeasured "+entry)
	}
	for _, d := range drift {
		lines = append(lines, checkPrefix+"drift "+d.Path+" "+d.Cause+": run "+d.Fix)
	}
	for _, h := range open {
		lines = append(lines, fmt.Sprintf("%shole open %s:%d %s: fill it and its red test", checkPrefix, h.Path, h.Line, h.ID))
	}
	if len(drift)+len(open)+len(unmeasured) == 0 {
		return append(lines, checkPrefix+"clean, 0 drift, 0 open holes")
	}
	return append(lines, fmt.Sprintf("%s%d drift, %d open holes", checkPrefix, len(drift), len(open)))
}

// personCause is the pt-BR form of a drift cause; a cause without one
// keeps its English words.
func personCause(cause string) string {
	switch cause {
	case gen.DriftDiffers:
		return "difere da sua renderização"
	case gen.DriftMissing:
		return "arquivo regenerável ausente"
	case gen.DriftContractGone:
		return "o contrato da entrada do manifesto sumiu"
	case gen.DriftThirdParty:
		return "difere do que o seu gerador fixado escreve"
	case gen.DriftStub:
		return "não tem os stubs do seu contrato"
	default:
		return cause
	}
}

// checkPersonLines are the person's pt-BR lines of golaw gen --check; an
// unmeasured entry keeps the tool's own words, which name what to install.
func checkPersonLines(drift []gen.Drift, open []holes.Hole, unmeasured []string) []string {
	lines := make([]string, 0, len(drift)+len(open)+len(unmeasured)+1)
	for _, entry := range unmeasured {
		lines = append(lines, checkPrefix+"não medido "+entry)
	}
	for _, d := range drift {
		lines = append(lines, checkPrefix+"deriva "+d.Path+" "+personCause(d.Cause)+": rode "+d.Fix)
	}
	for _, h := range open {
		lines = append(lines, fmt.Sprintf("%slacuna aberta %s:%d %s: preencha a lacuna e o seu teste vermelho", checkPrefix, h.Path, h.Line, h.ID))
	}
	if len(drift)+len(open)+len(unmeasured) == 0 {
		return append(lines, checkPrefix+"limpo, 0 derivas, 0 lacunas abertas")
	}
	return append(lines, fmt.Sprintf("%s%d derivas, %d lacunas abertas", checkPrefix, len(drift), len(open)))
}

// holesJSON is each hole as the JSON reports print it, never null.
func holesJSON(hs []holes.Hole) []holeJSON {
	out := make([]holeJSON, 0, len(hs))
	for _, h := range hs {
		out = append(out, holeJSON{Path: h.Path, Line: h.Line, ID: h.ID, Constraint: h.Constraint})
	}
	return out
}

// genDocument is the JSON report of one golaw gen run.
func genDocument(res gen.Result, report apply.Report, notices []string) genJSON {
	doc := genJSON{
		Written: append([]string{}, report.Written...), Skipped: []skipJSON{},
		Opened: holesJSON(res.Opened), Closed: holesJSON(res.Closed), Removed: make([]removedJSON, 0, len(res.Removed)),
		Decisions: decisionsJSON(res.Decisions), Notices: append([]string{}, notices...),
	}
	for _, skip := range report.Skipped {
		doc.Skipped = append(doc.Skipped, skipJSON(skip))
	}
	for i := range res.Removed {
		doc.Removed = append(doc.Removed, removedJSON{Kind: res.Removed[i].Kind, Name: res.Removed[i].Name})
	}
	return doc
}

func decisionsJSON(ds []genkind.Decision) []decisionJSON {
	out := make([]decisionJSON, 0, len(ds))
	for _, d := range ds {
		out = append(out, decisionJSON(d))
	}
	return out
}

// checkDocument is the JSON report of one golaw gen --check run.
func checkDocument(drift []gen.Drift, open []holes.Hole, unmeasured []string) checkJSON {
	doc := checkJSON{Drift: make([]driftJSON, 0, len(drift)), Holes: holesJSON(open), Unmeasured: append([]string{}, unmeasured...)}
	for _, d := range drift {
		doc.Drift = append(doc.Drift, driftJSON(d))
	}
	return doc
}

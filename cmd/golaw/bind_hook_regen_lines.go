package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen"
	"github.com/cristianargotti/golaw/internal/gen/apply"
	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// maxRegenFileLines bounds the lines one file gives the agent (M6.6.3), so
// a large regeneration never floods the context; golaw gen --check names
// the rest.
const maxRegenFileLines = 20

// regenLine is one line in the agent's form and the person's, keyed by the
// file it is about; the summary has no file and is never bounded.
type regenLine struct {
	path, agent, person string
}

// regenLines writes the report of a regeneration: the lines in the order
// the agent acts on them, then the notices. The constraint, the decision
// text and the causes stay in English (decision 6).
func regenLines(language string, o regenOutcome) hook.RegenReport {
	lines := make([]regenLine, 0, 1+len(o.result.Open)+len(o.result.Closed)+len(o.result.Decisions)+len(o.skipped)+len(o.drift)+len(o.fixed))
	lines = append(lines, summaryLine(o)...)
	lines = append(lines, holeLines(o.result)...)
	lines = append(lines, decisionLines(o.result.Decisions)...)
	lines = append(lines, skipLines(o.skipped)...)
	lines = append(lines, driftLines(o.drift, o.driftContracts)...)
	lines = append(lines, fixedFileLines(o.fixed)...)
	return regenReport(language, bounded(lines), noticeLines(o))
}

// deferredLines is the report of an event that found the generation lock
// held: the regeneration is left to golaw gen, never waited for.
func deferredLines(language string) hook.RegenReport {
	return regenReport(language, nil, []regenLine{{agent: "regeneration deferred: run golaw gen", person: "golaw adiou a regeneração: rode golaw gen."}})
}

// regenReport spells the person's forms in pt-BR and reads the agent's
// lines for any other language.
func regenReport(language string, lines, notices []regenLine) hook.RegenReport {
	var r hook.RegenReport
	for _, l := range lines {
		r.Lines = append(r.Lines, l.agent)
	}
	for _, n := range notices {
		r.Notices = append(r.Notices, n.agent)
	}
	if language != model.DefaultLanguage {
		r.Person = append(slices.Clone(r.Lines), r.Notices...)
		return r
	}
	for _, l := range append(slices.Clone(lines), notices...) {
		r.Person = append(r.Person, l.person)
	}
	return r
}

func summaryLine(o regenOutcome) []regenLine {
	if len(o.contracts) == 0 {
		return nil
	}
	named := make([]string, 0, len(o.contracts))
	for _, c := range o.contracts {
		named = append(named, c+o.labels[c])
	}
	from := strings.Join(named, ", ")
	return []regenLine{{
		agent:  fmt.Sprintf("golaw regenerated from %s: %d files, %d drift", from, o.written, len(o.skipped)),
		person: fmt.Sprintf("golaw regenerou a partir de %s: %d arquivos, %d com deriva", from, o.written, len(o.skipped)),
	}}
}

func holeLines(res gen.Result) []regenLine {
	out := make([]regenLine, 0, len(res.Open)+len(res.Closed))
	for _, h := range res.Open {
		rest := strings.TrimSpace(fmt.Sprintf("%s:%d %s %s", h.Path, h.Line, h.ID, h.Constraint))
		out = append(out, regenLine{path: h.Path, agent: "hole open " + rest, person: "lacuna aberta " + rest})
	}
	for _, h := range res.Closed {
		rest := h.Path + " " + h.ID
		out = append(out, regenLine{path: h.Path, agent: "hole closed " + rest, person: "lacuna fechada " + rest})
	}
	return out
}

func decisionLines(decisions []genkind.Decision) []regenLine {
	out := make([]regenLine, 0, len(decisions))
	for _, d := range decisions {
		at := d.Path
		if d.Line > 0 {
			at = fmt.Sprintf("%s:%d", d.Path, d.Line)
		}
		rest := at + " " + d.Text
		out = append(out, regenLine{path: d.Path, agent: "decision " + rest, person: "decisão " + rest})
	}
	return out
}

func skipLines(skipped []apply.Skip) []regenLine {
	out := make([]regenLine, 0, len(skipped))
	for _, s := range skipped {
		rest := s.Path + ": " + s.Cause
		out = append(out, regenLine{path: s.Path, agent: "skipped " + rest, person: "arquivo pulado " + rest})
	}
	return out
}

// driftCause spells each drift cause of the engine for the person.
func driftCause(cause string) string {
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
	case gen.DriftRemoved:
		return "pertence a uma entrada cujos arquivos próprios foram apagados"
	default:
		return cause
	}
}

// driftLines names each drifted file, its cause and its fix: the contract
// to edit when the manifest names one, then the command.
func driftLines(drift []gen.Drift, contracts map[string]string) []regenLine {
	out := make([]regenLine, 0, len(drift))
	for _, d := range drift {
		fix := d.Fix
		if fix == "" {
			fix = gen.FixGen
		}
		agent, person := "run "+fix, "rode "+fix
		if c := contracts[d.Path]; c != "" {
			agent, person = "edit "+c+", or "+agent, "edite "+c+", ou "+person
		}
		out = append(out, regenLine{
			path:   d.Path,
			agent:  fmt.Sprintf("drift %s %s: %s", d.Path, d.Cause, agent),
			person: fmt.Sprintf("deriva %s %s: %s", d.Path, driftCause(d.Cause), person),
		})
	}
	return out
}

func fixedFileLines(fixed []fixedFile) []regenLine {
	var out []regenLine
	for _, f := range fixed {
		if len(f.parts) > 0 {
			parts := strings.Join(f.parts, ", ")
			out = append(out, regenLine{path: f.path, agent: "formatted " + f.path + ": " + parts, person: "arquivo formatado " + f.path + ": " + parts})
		}
		if f.parse != "" {
			out = append(out, regenLine{path: f.path, agent: "parse " + f.parse, person: "erro de sintaxe " + f.parse})
		}
		if f.skipped != "" {
			rest := f.path + ": " + f.skipped
			out = append(out, regenLine{path: f.path, agent: "skipped " + rest, person: "arquivo pulado " + rest})
		}
		for _, line := range f.report {
			rest := f.path + ": " + line
			out = append(out, regenLine{path: f.path, agent: "imports " + rest, person: "importações " + rest})
		}
	}
	return out
}

func noticeLines(o regenOutcome) []regenLine {
	out := make([]regenLine, 0, len(o.causes)+len(o.outside))
	for _, cause := range o.causes {
		out = append(out, regenLine{
			agent:  "not regenerated in the hook: run golaw gen (" + cause + ")",
			person: "golaw não regenerou no hook: rode golaw gen (" + cause + ").",
		})
	}
	for _, name := range o.outside {
		out = append(out, regenLine{
			agent:  "run golaw gen: " + name + " regenerates converters outside the hook",
			person: "rode golaw gen: o " + name + " regenera os conversores fora do hook.",
		})
	}
	return out
}

// bounded keeps at most maxRegenFileLines lines per file and counts the
// rest in one closing line per file, in the order the files overflowed.
func bounded(lines []regenLine) []regenLine {
	seen := make(map[string]int)
	var out []regenLine
	var over []string
	for _, l := range lines {
		if l.path == "" {
			out = append(out, l)
			continue
		}
		seen[l.path]++
		if seen[l.path] <= maxRegenFileLines {
			out = append(out, l)
		} else if seen[l.path] == maxRegenFileLines+1 {
			over = append(over, l.path)
		}
	}
	for _, p := range over {
		n := seen[p] - maxRegenFileLines
		out = append(out, regenLine{
			path:   p,
			agent:  fmt.Sprintf("%s: %d more lines; golaw gen --check names them all", p, n),
			person: fmt.Sprintf("%s: mais %d linhas; golaw gen --check nomeia todas", p, n),
		})
	}
	return out
}

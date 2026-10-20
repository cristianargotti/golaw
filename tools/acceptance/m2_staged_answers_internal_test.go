package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	scanreport "github.com/cristianargotti/golaw/internal/report"
)

// m2PolicyRefusal is the refusal of a weakened staged policy: exit 2, no
// report on stdout, the key and the repair on stderr.
const m2PolicyRefusal = "golaw: golaw.yml at the index weakens checks.sizes.file_lines from 300 to 301: " +
	"restore what the policy in force holds, or unstage golaw.yml with git restore --staged golaw.yml\n"

// m2StagedScript answers the one staged scan of a staged row with the
// answer the row's table sets, never one derived from the candidate; every
// other command builds the row's state through the fixture executor.
func m2StagedScript(t *testing.T, s *suite, name, mode string) *m2Script {
	t.Helper()
	return newM2Script(t, s, m2StagedAnswer(t, s, name, mode))
}

// m2StagedAnswer answers the exact scan --staged --json of the candidate:
// the red answer for a red row and the clean answer for a near row, the two
// swapped when the mode corrupts; mutation also rewrites note.txt.
func m2StagedAnswer(t *testing.T, s *suite, name, mode string) m2Answer {
	t.Helper()
	return func(_ context.Context, c command) (m2Reply, bool) {
		if c.program != s.binary || !slices.Equal(c.args, []string{commandScan, "--staged", "--json"}) {
			return m2Reply{}, false
		}
		if mode == "broken" {
			return m2Reply{got: result{err: ErrAcceptance}, target: true}, true
		}
		red := strings.HasSuffix(name, "-red") != (mode == "corrupt")
		got := m2StagedDocument(t, name, red)
		if mode == "mutation" {
			m2Write(t, filepath.Join(c.dir, "note.txt"), "unexpected mutation\n")
		}
		return m2Reply{got: got, target: true}, true
	}
}

// m2StagedDocument is the answer of a staged row: the policy refusal, the
// report of the row's one introduced finding, or the clean report.
func m2StagedDocument(t *testing.T, name string, red bool) result {
	t.Helper()
	family, _, _ := strings.Cut(name, "-")
	if family == "policy" && red {
		return result{code: 2, stderr: m2PolicyRefusal}
	}
	doc := scanreport.Document{
		Schema: 1, KitVersion: m2Version, Mode: "staged", Verdict: "clean",
		Findings: []model.Finding{}, Checks: []scanreport.Status{},
	}
	code := 0
	if red {
		doc.Verdict, code = "fail", 1
		doc.Findings = []model.Finding{m2StagedFinding(family)}
		doc.Counts = scanreport.Counts{Findings: 1, Enforced: 1}
	}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return result{stdout: string(data) + "\n", code: code}
}

// m2StagedFinding is the finding a red row introduces: the file length of
// the length rows, and the em dash of the text rows at the row's path.
func m2StagedFinding(family string) model.Finding {
	if family == "length" {
		return model.Finding{
			Check: "goast.file-length", Rule: "rules/4.3@1", Path: "big.go",
			Code: "file-length", Severity: model.SeverityFail,
			Message:     "file has 301 lines, the limit is 300: split it by responsibility",
			Fingerprint: strings.Repeat("e", 64),
		}
	}
	paths := map[string]string{"deletion": "new.txt", "rename": "new.txt", "dirty": "dirty.txt"}
	path, found := paths[family]
	if !found {
		path = "note.txt"
	}
	return model.Finding{
		Check: "text.dashes", Rule: "rules/4.1@1", Path: path, Line: 2,
		Code: "em-dash", Severity: model.SeverityFail,
		Message:     "em dash (U+2014) in text; use a comma, a colon or a hyphen",
		Fingerprint: strings.Repeat("f", 64),
	}
}

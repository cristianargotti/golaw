package genguard_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/genguard"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
	"github.com/cristianargotti/golaw/internal/source"
)

const manifestReason = "is written only by the golaw CLI: run golaw new <kind> <name> or golaw gen."

// runCheck judges one candidate with the check the way the hook folds it
// over the base.
func runCheck(t *testing.T, c model.Check, tc writeCase) model.Result {
	t.Helper()
	base := map[string][]byte{}
	for p, text := range tc.base {
		base[p] = []byte(text)
	}
	candidate := model.Candidate{Path: tc.path, Op: tc.op, From: tc.from, Reconstructed: true}
	content := map[string][]byte{}
	if tc.op != model.CandidateDelete {
		content[tc.path] = []byte(tc.content)
	}
	overlay, candidates, err := source.NewOverlay(proof.NewTree(base), source.FromCandidates([]model.Candidate{candidate}, content))
	if err != nil {
		t.Fatal(err)
	}
	event := &model.Event{Shape: model.ShapeWrite, Agent: "tester", Tool: "Write", Candidates: candidates}
	res, err := c.Run(t.Context(), model.Input{Mode: model.ModeHook, Policy: proof.Policy(), Snapshot: overlay, Event: event})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestManifestWriteDeniesEveryAgentWriteOfTheManifest(t *testing.T) {
	t.Parallel()
	gen := manifestYAML(t)
	tests := map[string]struct {
		tc     writeCase
		denied bool
	}{
		"red: a replace":                {writeCase{base: map[string]string{manifest.Path: gen}, op: model.CandidateReplace, path: manifest.Path, content: "golaw-gen: 1\n"}, true},
		"red: a delete":                 {writeCase{base: map[string]string{manifest.Path: gen}, op: model.CandidateDelete, path: manifest.Path}, true},
		"red: a move away":              {writeCase{base: map[string]string{manifest.Path: gen}, op: model.CandidateMove, path: ".golaw/old.yml", from: manifest.Path, content: gen}, true},
		"red: a create":                 {writeCase{op: model.CandidateCreate, path: manifest.Path, content: "golaw-gen: 1\n"}, true},
		"near: the same name elsewhere": {writeCase{op: model.CandidateCreate, path: "docs/gen.yml", content: "golaw-gen: 1\n"}, false},
		"near: another case on a case sensitive system": {writeCase{op: model.CandidateCreate, path: ".golaw/GEN.yml", content: "golaw-gen: 1\n"}, false},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			res := runCheck(t, genguard.ManifestWrite(), tt.tc)
			if !tt.denied {
				if len(res.Findings) != 0 {
					t.Fatalf("findings = %+v", res.Findings)
				}
				return
			}
			if len(res.Findings) != 1 || res.Findings[0].Path != manifest.Path || res.Findings[0].Code != "manifest-write" || res.Findings[0].Message != manifestReason {
				t.Fatalf("findings = %+v", res.Findings)
			}
		})
	}
}

package genguard_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/genguard"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

const (
	streamPath   = "internal/adapters/out/vendor/stream.go"
	bareMarker   = "// golaw:handwritten\npackage vendor\n"
	reasonMarker = "// golaw:handwritten the vendor API streams a chunked body\npackage vendor\n"
	bareReason   = "adds golaw:handwritten without a reason: write why after the marker, for example // golaw:handwritten the vendor API streams a chunked body."
)

func TestHandwrittenReasonDeniesABareMarkerTheCandidateIntroduces(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		tc     writeCase
		denied bool
	}{
		"red: a new file with a bare marker":  {writeCase{op: model.CandidateCreate, path: streamPath, content: bareMarker}, true},
		"red: an edit that strips the reason": {writeCase{base: map[string]string{streamPath: reasonMarker}, op: model.CandidateReplace, path: streamPath, content: bareMarker}, true},
		"near: a marker with its reason":      {writeCase{op: model.CandidateCreate, path: streamPath, content: reasonMarker}, false},
		"near: a bare marker the base holds":  {writeCase{base: map[string]string{streamPath: bareMarker}, op: model.CandidateReplace, path: streamPath, content: bareMarker + "\nconst x = 1\n"}, false},
		"near: the marker in a markdown file": {writeCase{op: model.CandidateCreate, path: "docs/escape.md", content: "// golaw:handwritten\n"}, false},
		"near: a delete":                      {writeCase{base: map[string]string{streamPath: bareMarker}, op: model.CandidateDelete, path: streamPath}, false},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			res := runCheck(t, genguard.HandwrittenReason(), tt.tc)
			if !tt.denied {
				if len(res.Findings) != 0 {
					t.Fatalf("findings = %+v", res.Findings)
				}
				return
			}
			f := res.Findings
			if len(f) != 1 || f[0].Path != streamPath || f[0].Line != 1 || f[0].Code != "handwritten-reason" || f[0].Message != bareReason || f[0].Severity != model.SeverityDeny {
				t.Fatalf("findings = %+v", f)
			}
		})
	}
}

func TestHandwrittenNotesEveryMarkedFileInScan(t *testing.T) {
	t.Parallel()
	files := map[string][]byte{
		streamPath:                               []byte(reasonMarker),
		"internal/adapters/out/vendor/bare.go":   []byte("// golaw:handwritten\npackage vendor\n"),
		"internal/adapters/out/vendor/client.go": []byte("package vendor\n\n// golaw:handwritten is the escape of M7.\n"),
	}
	tree := proof.NewTree(files)
	in := model.Input{Mode: model.ModeScan, Policy: proof.Policy(), Snapshot: tree, Targets: tree.Files()}
	res, err := genguard.Handwritten().Run(t.Context(), in)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		streamPath:                             "is marked golaw:handwritten: the vendor API streams a chunked body",
		"internal/adapters/out/vendor/bare.go": "is marked golaw:handwritten without a reason: write why after the marker",
	}
	if len(res.Findings) != len(want) {
		t.Fatalf("findings = %+v", res.Findings)
	}
	for _, f := range res.Findings {
		if want[f.Path] != f.Message || f.Line != 1 || f.Code != "handwritten" || f.Severity != model.SeverityNote {
			t.Errorf("finding = %+v", f)
		}
	}
	hook := model.Input{Mode: model.ModeHook, Policy: proof.Policy(), Snapshot: tree, Targets: tree.Files(), Event: &model.Event{Shape: model.ShapeWrite, Agent: "tester", Tool: "Write", Candidates: []model.Candidate{{Path: streamPath, Op: model.CandidateReplace, Reconstructed: true}}}}
	if res, err := genguard.Handwritten().Run(t.Context(), hook); err != nil || res.Status.Kind != model.NotApplicable("").Kind {
		t.Errorf("hook = %+v, %v", res, err)
	}
}

func TestHandwrittenLeavesTheFixturesOfTheKitOut(t *testing.T) {
	t.Parallel()
	const fixture = "internal/proof/testdata/escape/stream.go"
	tree := proof.NewTree(map[string][]byte{fixture: []byte(bareMarker)})
	in := model.Input{Mode: model.ModeStaged, Policy: proof.Policy(), Snapshot: tree, Targets: []model.File{{Path: fixture, Class: model.ClassFixture}, {Path: "absent.go", Class: model.ClassTracked}}}
	res, err := genguard.Handwritten().Run(t.Context(), in)
	if err != nil || len(res.Findings) != 0 || res.Status.Kind != model.Measured().Kind {
		t.Fatalf("res = %+v, %v", res, err)
	}
}

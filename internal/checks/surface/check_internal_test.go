package surface

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
	detect "github.com/cristianargotti/golaw/internal/surface"
)

func TestSonarKeyNamesTheAnalyzerLanguage(t *testing.T) {
	t.Parallel()
	want := map[detect.Language]string{
		detect.JavaScript: "javascript:S1", detect.JSX: "javascript:S1", detect.HTML: "javascript:S1",
		detect.TypeScript: "typescript:S1", detect.TSX: "typescript:S1", detect.Python: "python:S1",
		detect.Dockerfile: "docker:S1", detect.CloudFormation: "cloudformation:S1", detect.CSS: "css:S1",
		detect.Shell: "", detect.YAML: "", detect.Language("cobol"): "",
	}
	for language, key := range want {
		if got := sonarKey(language, "S1"); got != key {
			t.Errorf("sonarKey(%s) = %q, want %q", language, got, key)
		}
	}
}

func TestSeverityFollowsTheEventAndTheRuleSet(t *testing.T) {
	t.Parallel()
	parity, strict := proof.Policy(), proof.Policy()
	strict.Surface.Rules = model.SurfaceStrict
	write := &model.Event{Shape: model.ShapeWrite}
	post := &model.Event{Shape: model.ShapePost}
	tests := []struct {
		name string
		c    check
		in   model.Input
		want model.Severity
	}{
		{"certain in a scan", check{certain: true}, model.Input{Mode: model.ModeScan, Policy: parity}, model.SeverityDeny},
		{"certain before a write", check{certain: true}, model.Input{Mode: model.ModeHook, Policy: parity, Event: write}, model.SeverityDeny},
		{"certain after a write", check{certain: true}, model.Input{Mode: model.ModeHook, Policy: parity, Event: post}, model.SeverityNote},
		{"feedback in the hook", check{}, model.Input{Mode: model.ModeHook, Policy: parity, Event: write}, model.SeverityNote},
		{"feedback in a scan", check{}, model.Input{Mode: model.ModeScan, Policy: parity}, model.SeverityFail},
		{"strict feedback under parity", check{strict: true}, model.Input{Mode: model.ModeScan, Policy: parity}, model.SeverityNote},
		{"strict feedback under strict", check{strict: true}, model.Input{Mode: model.ModeStaged, Policy: strict}, model.SeverityFail},
	}
	for _, tt := range tests {
		if got := tt.c.severity(tt.in); got != tt.want {
			t.Errorf("%s: severity = %s, want %s", tt.name, got, tt.want)
		}
	}
}

func TestReadsDecidesFromTheEntryAlone(t *testing.T) {
	t.Parallel()
	c := check{languages: []detect.Language{detect.CloudFormation, detect.Python}}
	goFiles := check{languages: []detect.Language{detect.Python}, goFiles: true}
	tests := []struct {
		c    check
		file model.File
		want bool
	}{
		{c, model.File{Path: "infra/stack.yaml", Class: model.ClassTracked}, true},
		{c, model.File{Path: "infra/stack.json", Class: model.ClassUntracked}, true},
		{c, model.File{Path: "bin/tool", Class: model.ClassTracked}, true},
		{c, model.File{Path: "a.py", Class: model.ClassGenerated}, false},
		{c, model.File{Path: "testdata/a.py", Class: model.ClassTracked}, false},
		{c, model.File{Path: "a.js", Class: model.ClassTracked}, false},
		{c, model.File{Path: "README.md", Class: model.ClassTracked}, false},
		{c, model.File{Path: "main.go", Class: model.ClassTracked}, false},
		{goFiles, model.File{Path: "main.go", Class: model.ClassTest}, true},
	}
	for _, tt := range tests {
		if got := tt.c.reads(&tt.file); got != tt.want {
			t.Errorf("reads(%s) = %t, want %t", tt.file.Path, got, tt.want)
		}
	}
}

func TestDescribeReadsABrokenTemplateAsYAML(t *testing.T) {
	t.Parallel()
	info := describe("infra/stack.yaml", []byte("AWSTemplateFormatVersion: \"2010-09-09\"\nResources: [\n"))
	if info.Language != detect.YAML || info.Class != detect.ClassSource {
		t.Fatalf("info = %+v, want plain yaml source", info)
	}
}

func TestJudgesKeepsGoFilesOnlyWhenAsked(t *testing.T) {
	t.Parallel()
	info := detect.FileInfo{Path: "main.go"}
	if (check{}).judges(info) || !(check{goFiles: true}).judges(info) {
		t.Fatal("a go file is judged only by a check that reads go files")
	}
}

// failing is a snapshot whose reads fail as an unreadable file does.
type failing struct{ *proof.Tree }

func (failing) Read(name string) ([]byte, error) {
	return nil, fmt.Errorf("%w: %s", model.ErrUnreadable, name)
}

func TestRunIsUnmeasuredWhenAFileCannotBeRead(t *testing.T) {
	t.Parallel()
	snapshot := failing{proof.NewTree(map[string][]byte{"a.js": []byte("eval(x);\n")})}
	in := model.Input{Mode: model.ModeScan, Policy: proof.Policy(), Snapshot: snapshot, Targets: snapshot.Files()}
	res, err := CodeEval().Run(t.Context(), in)
	if err != nil || res.Status.Kind != model.StatusUnmeasured {
		t.Fatalf("result = %+v, %v, want unmeasured", res, err)
	}
}

func TestRunStopsAtThePassedDeadline(t *testing.T) {
	t.Parallel()
	snapshot := proof.NewTree(map[string][]byte{"a.js": []byte("eval(x);\n")})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	in := model.Input{Mode: model.ModeScan, Policy: proof.Policy(), Snapshot: snapshot, Targets: snapshot.Files()}
	if _, err := CodeEval().Run(ctx, in); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want the canceled context", err)
	}
}

func TestThePartialViewKeepsTheNarrowListing(t *testing.T) {
	t.Parallel()
	view := partialView{Snapshot: proof.NewTree(map[string][]byte{"a.js": nil, "b.py": nil}), partial: map[string]bool{"a.js": true}}
	isPython := func(p string) bool { return p == "b.py" }
	if got := view.Select(isPython); len(got) != 1 || !view.Any(isPython) {
		t.Fatalf("select = %v, want b.py alone", got)
	}
	if _, err := view.Read("a.js"); !errors.Is(err, model.ErrTooLarge) {
		t.Fatalf("err = %v, want the read bound", err)
	}
}

// overBase folds a candidate tree over a base snapshot.
type overBase struct {
	*proof.Tree

	base model.Snapshot
}

func (o overBase) Base() model.Snapshot { return o.base }

func TestIntroducedReadsTheBaseOfAMoveAndKeepsNoneOfAnUnreadableBase(t *testing.T) {
	t.Parallel()
	candidate := proof.NewTree(map[string][]byte{"b.js": []byte("eval(one);\n")})
	move := &model.Event{Shape: model.ShapeWrite, Candidates: []model.Candidate{
		{Path: "b.js", Op: model.CandidateMove, From: "a.js", Reconstructed: true},
	}}
	tests := []struct {
		name string
		base model.Snapshot
		want int
	}{
		{"a moved legacy call", proof.NewTree(map[string][]byte{"a.js": []byte("eval(one);\n")}), 0},
		{"a move of a clean file", proof.NewTree(map[string][]byte{"a.js": []byte("run();\n")}), 1},
		{"an unreadable base", failing{proof.NewTree(map[string][]byte{"a.js": nil})}, 0},
	}
	for _, tt := range tests {
		snapshot := overBase{Tree: candidate, base: tt.base}
		in := model.Input{Mode: model.ModeHook, Policy: proof.Policy(), Snapshot: snapshot, Event: move, Targets: candidate.Files()}
		res, err := CodeEval().Run(t.Context(), in)
		if err != nil || len(res.Findings) != tt.want {
			t.Errorf("%s: findings = %+v, %v, want %d", tt.name, res.Findings, err, tt.want)
		}
	}
}

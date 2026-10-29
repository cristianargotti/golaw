package hook

import (
	"fmt"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
	detect "github.com/cristianargotti/golaw/internal/surface"
)

func TestFeedbackLinesNameTheRuleAndTheSonarKey(t *testing.T) {
	t.Parallel()
	got := feedbackLines([]model.Finding{
		{Check: "lint.ruff", Rule: "ruff/S105", Path: "b.py", Line: 3, Code: "s105", Message: "Possible hardcoded secret", Sonar: "python:S2068"},
		{Check: "docker.final-user", Rule: "rules/4.16@1", Path: "Dockerfile", Line: 1, Code: "final-user", Message: "the final stage sets no USER.", Sonar: "docker:S6471"},
		{Check: "lint.shellcheck", Rule: "shellcheck/SC2086", Path: "a.sh", Line: 4, Code: "sc2086", Message: "Double quote to prevent globbing and word splitting."},
		{Check: "surface.python-function-length", Rule: "rules/4.16@1", Path: "b.py", Code: "python-function-length", Message: "whole file."},
	})
	want := []string{
		"Dockerfile:1 docker.final-user (docker:S6471) the final stage sets no USER.",
		"a.sh:4 shellcheck/SC2086 Double quote to prevent globbing and word splitting.",
		"b.py surface.python-function-length whole file.",
		"b.py:3 ruff/S105 (python:S2068) Possible hardcoded secret",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("lines = %q\nwant %q", got, want)
	}
}

func TestFeedbackLinesCountWhatPassesTwentyPerFile(t *testing.T) {
	t.Parallel()
	var findings []model.Finding
	for line := 1; line <= 23; line++ {
		findings = append(findings, model.Finding{Check: "lint.ruff", Rule: "ruff/F401", Path: "a.py", Line: line, Code: "f401", Message: "unused"})
	}
	got := feedbackLines(findings)
	if len(got) != 21 || got[19] != "a.py:20 ruff/F401 unused" || got[20] != "a.py: 3 more findings; golaw lint --surface names them all" {
		t.Fatalf("lines = %q", got)
	}
}

func TestToolFilesKeepTwentySourceFilesOfASurfaceLanguage(t *testing.T) {
	t.Parallel()
	files := map[string][]byte{"main.go": []byte("package main\n"), "notes.md": []byte("why\n"), "views/app.min.js": []byte("x()\n")}
	for i := 1; i <= 22; i++ {
		files[fmt.Sprintf("scripts/s%02d.sh", i)] = []byte("#!/bin/sh\necho ok\n")
	}
	snapshot := proof.NewTree(files)
	got, gaps := toolFiles(model.Input{Mode: model.ModeHook, Policy: proof.Policy(), Snapshot: snapshot, Targets: snapshot.Files()})
	if len(got) != maxToolFiles || got[0].Path != "scripts/s01.sh" || got[0].Language != detect.Shell || string(got[0].Content) != "#!/bin/sh\necho ok\n" {
		t.Fatalf("files = %d, first %+v", len(got), got[0])
	}
	if len(gaps) != 2 || gaps[0].Path != "scripts/s21.sh" || gaps[1].Path != "scripts/s22.sh" {
		t.Fatalf("gaps = %+v, want the two files past the twentieth", gaps)
	}
}

func TestResultContextAndBlockingJoinTheLines(t *testing.T) {
	t.Parallel()
	r := Result{Reason: "goast.x: y (a.go)", Notice: "golaw did not measure a.sh: z; the pipeline judges it.", Lines: []string{"a.py:3 ruff/S105 x", "a.py:4 ruff/F401 y"}}
	if got := r.Context(); got != "golaw did not measure a.sh: z; the pipeline judges it.\na.py:3 ruff/S105 x\na.py:4 ruff/F401 y" {
		t.Errorf("context = %q", got)
	}
	if got := r.Blocking(); got != "goast.x: y (a.go)\na.py:3 ruff/S105 x\na.py:4 ruff/F401 y" {
		t.Errorf("blocking = %q", got)
	}
	if got := (Result{Reason: "r"}).Blocking(); got != "r" {
		t.Errorf("blocking without lines = %q", got)
	}
}

func TestCertainCheckIsTheListOfA27(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"surface.code-eval", "surface.hardcoded-credential", "surface.file-length", "surface.inline-script-size", "surface.suppression", "surface.analyzer-widening", "docker.tag-and-digest"} {
		if !certainCheck(id) {
			t.Errorf("%s is a certain check of A27", id)
		}
	}
	for _, id := range []string{"surface.html-prefer-tag", "surface.python-function-length", "docker.final-user", "cfn.bucket-https-only", "lint.ruff", "goast.function-length"} {
		if certainCheck(id) {
			t.Errorf("%s is feedback, not a certain check (near miss)", id)
		}
	}
}

func TestFeedbackHumanCountsTheLinesInTheLanguage(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		language string
		n        int
		want     string
	}{
		{model.DefaultLanguage, 2, "golaw enviou ao agente 2 apontamentos sobre o que ele escreveu; o pipeline julga o mesmo."},
		{"en", 1, "golaw sent the agent 1 finding about what it wrote; the pipeline judges the same."},
		{"en", 3, "golaw sent the agent 3 findings about what it wrote; the pipeline judges the same."},
	} {
		if got := feedbackHuman(tc.language, tc.n); got != tc.want {
			t.Errorf("%s %d = %q, want %q", tc.language, tc.n, got, tc.want)
		}
	}
}

func TestApplyKeepsAFindingAndAddsTheToolNotices(t *testing.T) {
	t.Parallel()
	f := fed{
		findings: []model.Finding{{Check: "lint.ruff", Rule: "ruff/F401", Path: "a.py", Line: 1, Message: "unused"}},
		notices:  []string{"ruff 0.14.0 is not installed."},
		person:   []string{"ruff 0.14.0 não está instalado."},
	}
	got := f.apply(Result{Outcome: Feedback, Reason: "goast.x: y (a.go)", Human: "Corrija."}, model.DefaultLanguage)
	if got.Reason != "goast.x: y (a.go) ruff 0.14.0 is not installed." || got.Human != "Corrija." || !slices.Equal(got.Lines, []string{"a.py:1 ruff/F401 unused"}) {
		t.Errorf("feedback = %+v", got)
	}
	cannot := Result{Outcome: Feedback, Reason: "cannot judge", CannotJudge: true}
	if got := f.apply(cannot, model.DefaultLanguage); got.Reason != cannot.Reason || got.Lines != nil {
		t.Errorf("cannot judge = %+v, want it unchanged", got)
	}
}

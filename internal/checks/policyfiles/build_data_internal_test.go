package policyfiles

import (
	"reflect"
	"testing"
)

func TestBuildDataKeepsLocalStagesOptional(t *testing.T) {
	t.Parallel()
	stages := []buildStage{
		{name: "pre-commit", argv: []string{"golaw", "hook", "pre-commit"}},
		{name: "commit-msg", argv: []string{"golaw", "hook", "commit-msg", "{1}"}},
		{name: "pre-push", argv: []string{"golaw", "hook", "pre-push", "{1}", "{2}"}},
	}
	if got := buildStages(); !reflect.DeepEqual(got, stages) {
		t.Fatalf("stages = %+v", got)
	}
	files := []buildFile{
		{path: "Makefile", required: true},
		{path: "lefthook.yml"},
	}
	if got := buildFiles(); !reflect.DeepEqual(got, files) {
		t.Fatalf("files = %+v", got)
	}
	if got := demandEvents(); !reflect.DeepEqual(got, []string{"schedule", "workflow_dispatch"}) {
		t.Fatalf("on-demand events = %v", got)
	}
	changed := buildStages()
	changed[0].argv[0] = "changed"
	if buildStages()[0].argv[0] != "golaw" {
		t.Fatal("stage terms share mutable storage")
	}
}

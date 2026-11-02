package main

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

func TestInitValuesCarryTheGenerateBlock(t *testing.T) {
	t.Parallel()
	support := "internal/platform/gensupport"
	config := policy.File{Kit: policy.Kit{Version: "0.3.0"}, Generate: &policy.Generate{Support: &support}}
	got := initValues(config, bootstrap.Layout{Main: bootstrap.MainCmd, Decisions: "docs/adr"}).Generate
	want := model.DefaultGenerate()
	want.Support = support
	want.Declared = true
	if got != want {
		t.Errorf("Values.Generate = %+v, want %+v", got, want)
	}
	if plain := initValues(policy.File{}, bootstrap.Layout{}).Generate; plain != model.DefaultGenerate() {
		t.Errorf("without a block = %+v, want the defaults", plain)
	}
}

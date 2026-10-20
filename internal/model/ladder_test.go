package model_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

func TestDefaultLadderArmsEveryRungButTheStagedTests(t *testing.T) {
	t.Parallel()
	want := model.Ladder{PreCommit: true, CommitMsg: true, PrePush: true}
	if got := model.DefaultLadder(); got != want {
		t.Errorf("DefaultLadder() = %+v, want %+v", got, want)
	}
	core := model.CoreDefaults()
	if core.Ladder != want {
		t.Errorf("CoreDefaults().Ladder = %+v, want %+v", core.Ladder, want)
	}
	if len(core.Waivers) != 0 {
		t.Errorf("CoreDefaults().Waivers = %+v, want none", core.Waivers)
	}
}

func TestEffectivePolicyCarriesTheWaiversAsCommitted(t *testing.T) {
	t.Parallel()
	waiver := model.Waiver{ID: "GO-2026-6603", Until: "2026-11-01", Record: "docs/adr/0007-go-1-27-2.md"}
	p := model.EffectivePolicy{Ladder: model.Ladder{PrePush: true}, Waivers: []model.Waiver{waiver}}
	if len(p.Waivers) != 1 || p.Waivers[0] != waiver || p.Ladder.PreCommit || !p.Ladder.PrePush {
		t.Errorf("policy = %+v", p)
	}
}

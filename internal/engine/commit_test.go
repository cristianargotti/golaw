package engine_test

import (
	"context"
	"errors"
	"testing"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
)

func commitInput(commit *model.Commit) model.Input {
	in := scanInput()
	in.Mode = model.ModeCommit
	in.Commit = commit
	return in
}

func TestDispatchRefusesACommitModeWithoutItsCommit(t *testing.T) {
	t.Parallel()
	reg := build(t)
	refused := map[string]model.Input{
		"commit mode without a commit": commitInput(nil),
		"an unresolved cleanup":        commitInput(&model.Commit{Message: "x", Cleanup: "default"}),
		"a commit in scan mode":        func() model.Input { in := scanInput(); in.Commit = &model.Commit{}; return in }(),
	}
	for name, in := range refused {
		if _, err := engine.Dispatch(t.Context(), reg, in); !errors.Is(err, model.ErrUsage) {
			t.Errorf("%s: %v, want ErrUsage", name, err)
		}
	}
}

func TestDispatchHandsTheCommitToACommitCheck(t *testing.T) {
	t.Parallel()
	var seen *model.Commit
	check := fake{spec: spec("commit.seen", model.ScopeCommit), run: func(_ context.Context, in model.Input) (model.Result, error) {
		seen = in.Commit
		return model.Result{Status: model.Measured(), Scope: "1 commit"}, nil
	}}
	commit := &model.Commit{Message: "feat: add the rung\n"}
	run, err := engine.Dispatch(t.Context(), build(t, check), commitInput(commit))
	if err != nil || seen != commit || run.Outcomes[0].Result.Status.Kind != model.StatusMeasured {
		t.Errorf("run = %+v, seen = %+v, err = %v", run, seen, err)
	}
}

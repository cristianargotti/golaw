package checks

import (
	"context"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

type named struct {
	id string
}

func (n named) Spec() model.Spec { return model.Spec{ID: n.id} }

func (named) Run(context.Context, model.Input) (model.Result, error) {
	return model.Result{}, nil
}

func TestConcatKeepsFamilyOrder(t *testing.T) {
	t.Parallel()
	text := func() []model.Check { return []model.Check{named{"text.a"}, named{"text.b"}} }
	shell := func() []model.Check { return []model.Check{named{"shell.a"}} }
	got := concat(text, shell)
	if len(got) != 3 || got[0].Spec().ID != "text.a" || got[2].Spec().ID != "shell.a" {
		t.Errorf("concat = %v", got)
	}
	if len(concat()) != 0 {
		t.Error("no family yields no check")
	}
	for _, family := range families() {
		if len(family()) == 0 {
			t.Error("a registered family must carry at least one check")
		}
	}
}

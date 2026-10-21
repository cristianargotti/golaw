package goast_test

import (
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/goast"
	"github.com/cristianargotti/golaw/internal/model"
)

// unparsable is the added lines of a hunk that did not apply: no Go file.
const unparsable = "\tif err != nil {\n\t\treturn err\n"

func hookInput(c model.Check, reconstructed bool) model.Input {
	in := input(c, map[string]string{"a.go": unparsable, "a_test.go": "package a\n"})
	in.Mode = model.ModeHook
	in.Targets = in.Targets[:1]
	in.Event = &model.Event{
		Agent: "probe", Tool: "patch", Shape: model.ShapeWrite,
		Candidates: []model.Candidate{{Path: "a.go", Op: model.CandidateEdit, Reconstructed: reconstructed}},
	}
	return in
}

func TestPartialCandidateIsLeftToThePostEvent(t *testing.T) {
	t.Parallel()
	for _, c := range []model.Check{goast.FunctionLength(), goast.FileLength(), goast.Goleak()} {
		partial, err := c.Run(t.Context(), hookInput(c, false))
		if err != nil || partial.Status.Kind != model.StatusMeasured || len(partial.Findings) != 0 {
			t.Fatalf("%s judged the added lines as a file: %+v, %v", c.Spec().ID, partial, err)
		}
		if c.Spec().ID != "goast.goleak" && !slices.Equal(partial.Unread, []string{"a.go"}) {
			t.Fatalf("%s left %v unread", c.Spec().ID, partial.Unread)
		}
	}
	whole, err := goast.FunctionLength().Run(t.Context(), hookInput(goast.FunctionLength(), true))
	if err != nil || whole.Status.Kind != model.StatusUnmeasured {
		t.Fatalf("a whole candidate that does not parse must stay unmeasured: %+v, %v", whole, err)
	}
}

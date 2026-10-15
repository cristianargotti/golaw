package proof_test

import (
	"errors"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

func TestNoOpAndAlwaysFinding(t *testing.T) {
	t.Parallel()
	in := model.Input{Mode: model.ModeScan, Policy: proof.Policy(), Snapshot: proof.NewTree(nil)}
	quiet := proof.NoOp(dashCheck{id: "text.dashes"})
	if quiet.Spec().ID != "text.dashes" {
		t.Error("NoOp must keep the spec")
	}
	res, err := quiet.Run(t.Context(), in)
	if err != nil || res.Status.Kind != model.StatusMeasured || len(res.Findings) != 0 {
		t.Errorf("NoOp.Run = %+v, %v", res, err)
	}
	loud := proof.AlwaysFinding(dashCheck{id: "text.dashes"}, "em-dash")
	res, err = loud.Run(t.Context(), in)
	if err != nil || len(res.Findings) != 1 || res.Findings[0].Code != "em-dash" || res.Findings[0].Path != "" {
		t.Errorf("AlwaysFinding without targets = %+v, %v", res, err)
	}
	in.Targets = []model.File{{Path: "a.md"}, {Path: "b.md"}}
	res, err = loud.Run(t.Context(), in)
	if err != nil || len(res.Findings) != 2 || res.Findings[1].Path != "b.md" || res.Findings[1].Line != 1 {
		t.Errorf("AlwaysFinding with targets = %+v", res.Findings)
	}
}

func TestRewire(t *testing.T) {
	t.Parallel()
	reg := registry(t, dashCheck{id: "text.dashes"}, noVerifyCheck{})
	wired, err := proof.Rewire(reg, "text.dashes", proof.NoOp)
	if err != nil {
		t.Fatal(err)
	}
	if wired.Len() != 2 || wired.String() != reg.String() {
		t.Errorf("rewired registry = %s", wired)
	}
	replaced, _ := wired.Lookup("text.dashes")
	if _, still := replaced.(dashCheck); still {
		t.Error("the named check must be replaced")
	}
	kept, _ := wired.Lookup("shell.no-verify")
	if _, same := kept.(noVerifyCheck); !same {
		t.Error("the other check must be kept")
	}
	if _, err := proof.Rewire(reg, "text.todo", proof.NoOp); !errors.Is(err, model.ErrUsage) {
		t.Errorf("unknown id: %v", err)
	}
	broken := func(model.Check) model.Check { return dashCheck{id: "Broken"} }
	if _, err := proof.Rewire(reg, "text.dashes", broken); !errors.Is(err, model.ErrInvalidSpec) {
		t.Errorf("invalid replacement: %v", err)
	}
}

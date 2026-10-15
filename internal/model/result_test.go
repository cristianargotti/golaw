package model_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

func TestStatusConstructors(t *testing.T) {
	t.Parallel()
	if s := model.Measured(); s.Kind != model.StatusMeasured || s.Reason != "" {
		t.Errorf("Measured() = %+v", s)
	}
	if s := model.NotApplicable("capability api is off"); s.Kind != model.StatusNotApplicable || s.Reason != "capability api is off" {
		t.Errorf("NotApplicable() = %+v", s)
	}
	if s := model.Unmeasured("deadline"); s.Kind != model.StatusUnmeasured || s.Reason != "deadline" {
		t.Errorf("Unmeasured() = %+v", s)
	}
	names := map[model.StatusKind]string{
		model.StatusMeasured: "measured", model.StatusNotApplicable: "not-applicable",
		model.StatusUnmeasured: "unmeasured", model.StatusKind(0): "unknown",
	}
	for kind, want := range names {
		if kind.String() != want {
			t.Errorf("StatusKind(%d).String() = %q, want %q", int(kind), kind.String(), want)
		}
		if kind.Valid() != (want != "unknown") {
			t.Errorf("StatusKind(%d).Valid() = %v", int(kind), kind.Valid())
		}
	}
}

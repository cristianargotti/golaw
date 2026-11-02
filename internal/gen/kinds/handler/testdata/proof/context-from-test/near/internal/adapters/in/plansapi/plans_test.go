package plansapi_test

import (
	"errors"
	"testing"

	apigen "example.com/slice/api/gen"
	"example.com/slice/internal/gensupport"
)

func TestGetPlanHoleIsOpen(t *testing.T) {
	t.Parallel()
	_, err := newTestHandler().GetPlan(t.Context(), apigen.GetPlanRequestObject{})
	if errors.Is(err, gensupport.ErrNotImplemented) {
		t.Fatal("hole plansapi.get-plan is open: map the request to the input of a use case of h.uc")
	}
}

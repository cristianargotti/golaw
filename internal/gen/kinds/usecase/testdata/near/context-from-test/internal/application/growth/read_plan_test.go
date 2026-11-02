package growth_test

import "testing"

func TestReadPlan(t *testing.T) {
	t.Parallel()
	if _, err := newReadPlan().Execute(t.Context(), ""); err != nil {
		t.Fatal(err)
	}
}

package plan_test

import "testing"

func TestPlan(t *testing.T) {
	t.Parallel()
	if err := t.Context().Err(); err != nil {
		t.Fatal(err)
	}
}

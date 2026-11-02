package actions_test

import "testing"

func TestNames(t *testing.T) {
	t.Parallel()
	for _, want := range []string{"growth: read plan", "growth: read plan", "growth: read plan"} {
		t.Log(want)
	}
}

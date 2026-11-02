package porttest

import "testing"

func TestZeroValuesOnTheTestContext(t *testing.T) {
	t.Parallel()
	if got := zeroValues(t.Context(), greeter(fake{rec: &script{}}), greetMethod()); len(got) != 0 {
		t.Errorf("problems = %q", got)
	}
}

package porttest

import (
	"context"
	"testing"
)

func TestZeroValuesOnARootContext(t *testing.T) {
	t.Parallel()
	if got := zeroValues(context.Background(), greeter(fake{rec: &script{}}), greetMethod()); len(got) != 0 {
		t.Errorf("problems = %q", got)
	}
}

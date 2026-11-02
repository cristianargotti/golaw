package plan_test

import (
	"context"
	"testing"
)

func TestPlan(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	if ctx.Err() != nil {
		t.Fatal(ctx.Err())
	}
}

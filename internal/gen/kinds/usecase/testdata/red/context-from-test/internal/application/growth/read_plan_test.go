package growth_test

import (
	"context"
	"testing"
)

func TestReadPlan(t *testing.T) {
	t.Parallel()
	if _, err := newReadPlan().Execute(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
}

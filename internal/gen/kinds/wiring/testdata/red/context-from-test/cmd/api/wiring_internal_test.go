package main

import (
	"context"
	"testing"
)

func TestServeHoleWiringInfra(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Serve(ctx); err != nil {
		t.Log(err)
	}
}

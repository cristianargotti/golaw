package main

import (
	"context"
	"fmt"

	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

func (b binding) setupDoctor(ctx context.Context, root string) ([]doctor.Line, error) {
	state, err := b.installState()
	if err != nil {
		return nil, err
	}
	ladder := model.DefaultLadder()
	if checkout, ok := policy.CheckoutLadder(root); ok {
		ladder = checkout
	}
	facts, err := doctor.CollectSetup(ctx, root, state, b.environ(), ladder)
	if err != nil {
		return nil, fmt.Errorf("collect setup diagnostics: %w", err)
	}
	return doctor.AssessSetup(facts), nil
}

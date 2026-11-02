package postgres

import (
	"context"

	"example.com/slice/internal/application/growth"
	"example.com/slice/internal/gensupport"
)

const actionPlansReadPlan = "postgres: plans read plan"

// ReadPlan reads the plan of one account.
func (p *Plans) ReadPlan(ctx context.Context, account growth.AccountID) (growth.Plan, error) {
	plan, err := planOf(p.queries.ReadPlan(ctx, string(account)))
	return plan, gensupport.Prefix(actionPlansReadPlan)(err)
}

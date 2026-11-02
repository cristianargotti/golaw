package postgres

import (
	"context"

	"example.com/slice/internal/application/growth"
	"example.com/slice/internal/gensupport"
)

// ReadPlan reads the plan of one account.
func (p *Plans) ReadPlan(ctx context.Context, account growth.AccountID) (growth.Plan, error) {
	row, err := p.queries.ReadPlan(ctx, string(account))
	if err != nil {
		return growth.Plan{}, gensupport.Prefix("postgres: plans read plan")(err)
	}
	plan, err := planOf(row, nil)
	if err != nil {
		return growth.Plan{}, gensupport.Prefix("postgres: plans read plan")(err)
	}
	return plan, gensupport.Prefix("postgres: plans read plan")(nil)
}

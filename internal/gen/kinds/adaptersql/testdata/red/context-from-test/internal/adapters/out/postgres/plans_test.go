package postgres_test

import (
	"context"
	"database/sql"
	"testing"

	"example.com/slice/internal/adapters/out/postgres"
	"example.com/slice/internal/application/growth"
	"example.com/slice/internal/gensupport/sqltest"
)

func TestPlansReadPlan(t *testing.T) {
	t.Parallel()
	db := sqltest.Open(t, sqltest.Step{Err: sql.ErrNoRows})
	var account growth.AccountID
	if _, err := postgres.NewPlans(db).ReadPlan(context.Background(), account); err == nil {
		t.Fatal("a missing plan answered no error")
	}
}

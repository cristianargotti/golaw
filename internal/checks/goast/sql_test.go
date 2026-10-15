package goast_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/goast"
	"github.com/cristianargotti/golaw/internal/model"
)

const querySource = `package pg

import "fmt"

func q(name, id string) []string {
	a := fmt.Sprintf("SELECT id FROM users WHERE name = '%s'", name)
	b := fmt.Sprint("DELETE FROM users WHERE id = ", id)
	c := "SELECT id FROM users WHERE id = " + id + " AND x = 1"
	d := "WHERE id = $1" + " AND name = $2"
	e := fmt.Sprintf("user %s", name)
	f := id + " rows"
	g := fmt.Sprintf(name)
	return []string{a, b, c, d, e, f, g}
}
`

func TestSQLFromStrings(t *testing.T) {
	t.Parallel()
	res := scan(t, goast.SQLFromStrings(), map[string]string{
		"sqlc.yaml":        "version: \"2\"\n",
		"internal/pg/q.go": querySource,
	})
	expectCodes(t, res, "sql-from-strings", "sql-from-strings", "sql-from-strings")
	expectLines(t, res, 6, 7, 8)
	formatted := "a query is formatted from a string: queries live in db/queries and go through sqlc"
	joined := "a query is concatenated from strings: queries live in db/queries and go through sqlc"
	if res.Findings[0].Message != formatted || res.Findings[2].Message != joined {
		t.Errorf("messages = %q, %q", res.Findings[0].Message, res.Findings[2].Message)
	}
}

func TestSQLFromStringsNeedsSqlc(t *testing.T) {
	t.Parallel()
	res := scan(t, goast.SQLFromStrings(), map[string]string{"internal/pg/q.go": querySource})
	expectNotApplicable(t, res, "sqlc.yaml is absent")
}

// expectNotApplicable fails unless the status is not applicable for the
// reason.
func expectNotApplicable(t *testing.T, res model.Result, reason string) {
	t.Helper()
	if res.Status.Kind != model.StatusNotApplicable || res.Status.Reason != reason {
		t.Fatalf("status = %s (%s), want not applicable with %q", res.Status.Kind, res.Status.Reason, reason)
	}
}

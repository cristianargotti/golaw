package contract_test

import (
	"reflect"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/contract"
	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gentest"
)

const sqlcgen = "internal/adapters/out/postgres/sqlcgen"

func TestQueriesReadTheMethodsOfTheSqlcOutput(t *testing.T) {
	t.Parallel()
	got, err := growthReader(t).Queries(sqlcgen)
	account := []genkind.Param{{Name: "account", Type: "string"}}
	want := []genkind.Query{
		{Name: "ArchivePlan", Params: account, Results: []genkind.Param{{Type: "int64"}, {Type: "error"}}, File: "plans.sql.go"},
		{Name: "ReadPlan", Params: account, Results: []genkind.Param{{Type: "Plan"}, {Type: "error"}}, File: "plans.sql.go"},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("Queries = %+v, %v\nwant      %+v", got, err, want)
	}
}

// Near rows: WithTx of db.go, a method of another receiver and a file of
// another directory are no queries; a method without a context keeps every
// parameter.
func TestQueriesKeepOnlyTheQueryMethods(t *testing.T) {
	t.Parallel()
	files := gentest.Contracts()
	files[sqlcgen+"/extra.sql.go"] = []byte("package sqlcgen\n\nfunc (q *Queries) Count(table string) (int64, error) { return 0, nil }\n\nfunc (r *Row) Scan() error { return nil }\n")
	files["internal/adapters/out/other/sqlcgen/x.sql.go"] = []byte("package sqlcgen\n\nfunc (q *Queries) Other() error { return nil }\n")
	got, err := contract.Reader{Snapshot: gentest.Snapshot(t, files)}.Queries(sqlcgen)
	names := make([]string, 0, len(got))
	for _, q := range got {
		names = append(names, q.Name+"@"+q.File)
	}
	if want := []string{"ArchivePlan@plans.sql.go", "Count@extra.sql.go", "ReadPlan@plans.sql.go"}; err != nil || !reflect.DeepEqual(names, want) {
		t.Errorf("Queries = %v, %v, want %v", names, err, want)
	}
	if got[1].Params[0].Name != "table" {
		t.Errorf("Count params = %+v", got[1].Params)
	}
	if empty, err := (contract.Reader{Snapshot: gentest.Snapshot(t, files)}).Queries("internal/absent"); err != nil || empty != nil {
		t.Errorf("a directory without output = %+v, %v", empty, err)
	}
}

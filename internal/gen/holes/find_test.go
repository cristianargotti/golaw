package holes_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/holes"
)

const plansPath = "internal/adapters/out/postgres/plans.go"

// ownedPlans is an owned file with one hole marked and called, one hole
// called only, and one filled method.
const ownedPlans = `package postgres

import (
	"context"

	"example.com/slice/internal/application/growth"
	"example.com/slice/internal/gensupport"
)

// ReadPlan reads the plan of one account.
func (p *Plans) ReadPlan(ctx context.Context, account growth.AccountID) (growth.Plan, error) {
	// golaw:hole plans.read-plan call p.queries.ReadPlan; map sql.ErrNoRows to growth.ErrPlanNotFound
	var zero growth.Plan
	return zero, gensupport.Hole("plans.read-plan", ctx, account)
}

// ArchivePlan archives the plan of one account.
func (p *Plans) ArchivePlan(ctx context.Context, account growth.AccountID) error {
	return gensupport.Hole("plans.archive-plan", ctx, account)
}

// PurgePlans is filled: its marker is gone and it calls no hole.
func (p *Plans) PurgePlans(ctx context.Context) error {
	return ctx.Err()
}
`

func TestFindReadsMarkersAndHoleCalls(t *testing.T) {
	t.Parallel()
	got, err := holes.Find(plansPath, []byte(ownedPlans))
	if err != nil {
		t.Fatal(err)
	}
	want := []holes.Hole{
		{
			ID: "plans.read-plan", Path: plansPath, Line: 12, Func: "(*Plans).ReadPlan",
			Constraint: "call p.queries.ReadPlan; map sql.ErrNoRows to growth.ErrPlanNotFound",
		},
		{ID: "plans.archive-plan", Path: plansPath, Line: 19, Func: "(*Plans).ArchivePlan"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Find = %+v\nwant   %+v", got, want)
	}
}

func TestFindNamesTheEnclosingDeclaration(t *testing.T) {
	t.Parallel()
	src := "package growth\n\n// Ledger stores the plans of an account.\ntype Ledger interface {\n" +
		"\t// golaw:hole ledger.methods declare the methods of Ledger, one line each with its doc comment\n}\n\n" +
		"func readPlan() {\n\t// golaw:hole growth.read-plan compute the plan\n}\n\n" +
		"// golaw:hole growth.file-level a marker outside any declaration\n"
	got, err := holes.Find("ports.go", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	funcs := make([]string, 0, len(got))
	for _, h := range got {
		funcs = append(funcs, h.ID+"@"+h.Func)
	}
	want := []string{"ledger.methods@Ledger", "growth.read-plan@readPlan", "growth.file-level@"}
	if !reflect.DeepEqual(funcs, want) {
		t.Errorf("holes = %v, want %v", funcs, want)
	}
}

// Red rows are holes; near rows look like one and are not.
func TestFindCountsOnlyCertainHoles(t *testing.T) {
	t.Parallel()
	const head = "package p\n\nimport (\n\t\"example.com/other\"\n\tgensupport \"example.com/slice/internal/support\"\n)\n\n"
	tests := map[string]struct {
		body string
		ids  []string
	}{
		"red: marker only":                {"func a() {\n\t// golaw:hole p.a do a\n}\n", []string{"p.a"}},
		"red: call only":                  {"func a() error {\n\treturn gensupport.Hole(\"p.a\")\n}\n", []string{"p.a"}},
		"red: marker and call count once": {"func a() error {\n\t// golaw:hole p.a do a\n\treturn gensupport.Hole(\"p.a\")\n}\n", []string{"p.a"}},
		"red: an explicitly named import": {"func a() error {\n\treturn gensupport.Hole(\"p.b\", 1)\n}\n", []string{"p.b"}},
		"red: the call of HoleOf":         {"func a() (int, error) {\n\treturn gensupport.HoleOf[int](\"p.c\")\n}\n", []string{"p.c"}},
		"near: another package's HoleOf":  {"func a() (int, error) {\n\treturn other.HoleOf[int](\"p.c\")\n}\n", nil},
		"near: a quoted marker":           {"var s = \"// golaw:hole p.a quoted\"\n", nil},
		"near: a block comment":           {"/* golaw:hole p.a in a block */\nvar s = 1\n", nil},
		"near: another package's Hole":    {"func a() error {\n\treturn other.Hole(\"p.a\")\n}\n", nil},
		"near: an unqualified Hole":       {"func Hole(string) error { return nil }\n\nfunc a() error {\n\treturn Hole(\"p.a\")\n}\n", nil},
		"near: an id that is no literal":  {"func a(id string) error {\n\treturn gensupport.Hole(id)\n}\n", nil},
		"near: a marker without an id":    {"func a() {\n\t// golaw:hole\n}\n", nil},
		"near: a malformed id":            {"func a() {\n\t// golaw:hole Plans.Read do it\n}\n", nil},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := holes.Find("p.go", []byte(head+tc.body))
			if err != nil {
				t.Fatal(err)
			}
			var ids []string //nolint:prealloc // a near row expects nil, as its ids column spells it
			for _, h := range got {
				ids = append(ids, h.ID)
			}
			if !reflect.DeepEqual(ids, tc.ids) {
				t.Errorf("ids = %v, want %v", ids, tc.ids)
			}
		})
	}
}

func TestFindNamesAFileThatDoesNotParse(t *testing.T) {
	t.Parallel()
	if _, err := holes.Find("bad.go", []byte("package p\n\nfunc (\n")); err == nil || !strings.Contains(err.Error(), "bad.go") {
		t.Errorf("Find = %v, want a parse error naming the file", err)
	}
}

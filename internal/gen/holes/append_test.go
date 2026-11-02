package holes_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/holes"
)

// handFormatted keeps a spacing gofmt would change, so the test proves the
// edit never reformats the bytes before the appended text.
const handFormatted = `package postgres

import (
	"context"

	"example.com/slice/internal/application/growth"
)

// ReadPlan reads the plan of one account.
func (p *Plans) ReadPlan(ctx context.Context, account growth.AccountID) (growth.Plan, error) {
	return p.read(ctx,   account) // hand formatting the engine keeps
}
`

const archiveStub = "// ArchivePlan archives the plan of one account.\n" +
	"func (p *Plans) ArchivePlan(ctx context.Context, account growth.AccountID) error {\n" +
	"\t// golaw:hole plans.archive-plan call p.queries.ArchivePlan\n" +
	"\treturn wrap(gensupport.Hole(\"plans.archive-plan\", ctx, account))\n}\n"

const appended = `package postgres

import (
	"context"
	"errors"

	"example.com/slice/internal/application/growth"
	"example.com/slice/internal/gensupport"
)

// ReadPlan reads the plan of one account.
func (p *Plans) ReadPlan(ctx context.Context, account growth.AccountID) (growth.Plan, error) {
	return p.read(ctx,   account) // hand formatting the engine keeps
}

// ArchivePlan archives the plan of one account.
func (p *Plans) ArchivePlan(ctx context.Context, account growth.AccountID) error {
	// golaw:hole plans.archive-plan call p.queries.ArchivePlan
	return wrap(gensupport.Hole("plans.archive-plan", ctx, account))
}
`

func archive() genkind.Append {
	return genkind.Append{
		Path:    "internal/adapters/out/postgres/plans.go",
		Decls:   [][]byte{[]byte(archiveStub)},
		Imports: []genkind.Import{{Path: "context"}, {Path: "example.com/slice/internal/gensupport"}, {Path: "errors"}},
		Holes:   []string{"plans.archive-plan"},
	}
}

func TestAppendAddsTheStubAndItsImportsAndKeepsEveryOtherByte(t *testing.T) {
	t.Parallel()
	got, err := holes.Append([]byte(handFormatted), archive())
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != appended {
		t.Fatalf("Append =\n%s\nwant\n%s", got, appended)
	}
	found, err := holes.Find("plans.go", got)
	if err != nil || len(found) != 1 || found[0].ID != "plans.archive-plan" || found[0].Func != "(*Plans).ArchivePlan" {
		t.Errorf("the appended hole = %+v, %v", found, err)
	}
	if _, err := holes.Append(got, archive()); !errors.Is(err, holes.ErrDeclared) || !strings.Contains(err.Error(), "Plans.ArchivePlan") {
		t.Errorf("a second append = %v, want ErrDeclared naming the method", err)
	}
}

func TestAppendWritesAnImportBlockWhereThereIsNone(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		content string
		imports []genkind.Import
		decl    string
		want    string
	}{
		"no import declaration": {
			"package p\n\nvar x = 1\n",
			[]genkind.Import{{Path: "fmt"}},
			"func F() string { return fmt.Sprint(x) }\n",
			"package p\n\nimport (\n\t\"fmt\"\n)\n\nvar x = 1\n\nfunc F() string { return fmt.Sprint(x) }\n",
		},
		"a one line import": {
			"package p\n\nimport \"fmt\"\n\nfunc F() { fmt.Println() }\n",
			[]genkind.Import{{Path: "os"}},
			"func G() { os.Exit(0) }\n",
			"package p\n\nimport (\n\t\"fmt\"\n\t\"os\"\n)\n\nfunc F() { fmt.Println() }\n\nfunc G() { os.Exit(0) }\n",
		},
		"a new group of another domain": {
			"package p\n\nimport (\n\t\"fmt\"\n)\n\nfunc F() { fmt.Println() }\n",
			[]genkind.Import{{Name: "x", Path: "github.com/acme/x"}},
			"func G() { x.Do() }\n",
			"package p\n\nimport (\n\t\"fmt\"\n\n\tx \"github.com/acme/x\"\n)\n\nfunc F() { fmt.Println() }\n\nfunc G() { x.Do() }\n",
		},
		"nothing to import": {
			"package p\n", nil, "func G() {}\n", "package p\n\nfunc G() {}\n",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := holes.Append([]byte(tc.content), genkind.Append{Path: "p.go", Decls: [][]byte{[]byte(tc.decl)}, Imports: tc.imports})
			if err != nil || string(got) != tc.want {
				t.Errorf("Append = %q, %v\nwant     %q", got, err, tc.want)
			}
		})
	}
}

// The red rows: an edit the engine cannot make append only is refused.
func TestAppendRefusesWhatIsNotAnAppend(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		content, decl string
		want          error
	}{
		"an existing function":        {"package p\n\nfunc F() {}\n", "func F() {}\n", holes.ErrDeclared},
		"an existing type":            {"package p\n\ntype T struct{}\n", "type T int\n", holes.ErrDeclared},
		"two declarations in one":     {"package p\n", "func F() {}\n\nfunc G() {}\n", holes.ErrAppend},
		"a declaration that is no Go": {"package p\n", "func (\n", holes.ErrAppend},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := holes.Append([]byte(tc.content), genkind.Append{Path: "p.go", Decls: [][]byte{[]byte(tc.decl)}}); !errors.Is(err, tc.want) {
				t.Errorf("Append = %v, want %v", err, tc.want)
			}
		})
	}
	if _, err := holes.Append([]byte("package p\n\nfunc (\n"), archive()); err == nil {
		t.Error("an owned file that does not parse answered no error")
	}
}

func TestNamesListsTheTopLevelKeys(t *testing.T) {
	t.Parallel()
	src := "package p\n\nconst A = 1\n\nvar (\n\tb = 2\n\t_ = 3\n)\n\ntype T[K any] struct{}\n\nfunc (t *T[K]) M() {}\n\nfunc (T[K]) N() {}\n\nfunc F() {}\n"
	got, err := holes.Names([]byte(src))
	if want := []string{"A", "F", "T", "T.M", "T.N", "b"}; err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("Names = %v, %v, want %v", got, err, want)
	}
}

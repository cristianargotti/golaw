package adaptersql_test

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/contract"
	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/kinds/adaptersql"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/gentest"
)

const (
	mainFile     = dir + "/plans.go"
	internalFile = dir + "/plans_internal_test.go"
)

// regenerate runs Regenerate over files with the port and entry in the manifest.
func regenerate(t *testing.T, files map[string][]byte, entry manifest.Entry) genkind.Output {
	t.Helper()
	out, err := adaptersql.Kind().Regenerate(t.Context(), input(t, files, portEntry(), entry), entry)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// withoutMapper answers the written instance with planOf deleted from the
// owned main file, as an agent that moved the conversion away leaves it.
func withoutMapper(t *testing.T) (map[string][]byte, manifest.Entry) {
	t.Helper()
	out := newAdapter(t, gentest.Contracts(), "Plans")
	files := written(gentest.Contracts(), out)
	before, _, found := bytes.Cut(files[mainFile], []byte("// planOf converts"))
	if !found {
		t.Fatalf("%s lacks the mapper:\n%s", mainFile, files[mainFile])
	}
	files[mainFile] = before
	return files, out.Entry
}

func TestRegenerateAppendsAFreshMapperToTheInternalTests(t *testing.T) {
	t.Parallel()
	files, entry := withoutMapper(t)
	regen := regenerate(t, files, entry)
	appends := make(map[string]genkind.Append)
	for _, a := range regen.Appends {
		appends[a.Path] = a
	}
	if a := appends[mainFile]; !slices.Equal(a.Holes, []string{"plans.plan-of"}) || !bytes.Contains(a.Decls[0], []byte("func planOf(row sqlcgen.Plan, err error)")) {
		t.Errorf("main append = %+v", a)
	}
	if a := appends[internalFile]; len(a.Decls) != 1 || !bytes.Contains(a.Decls[0], []byte("func TestPlanOf(t *testing.T)")) {
		t.Errorf("internal append = %+v", a)
	}
	if len(regen.Owned) != 0 || !slices.Equal(regen.Entry.Owned, entry.Owned) {
		t.Errorf("owned %+v, entry owned %v", regen.Owned, regen.Entry.Owned)
	}
}

func TestRegenerateCreatesTheInternalTestsAFreshMapperLacks(t *testing.T) {
	t.Parallel()
	files, entry := withoutMapper(t)
	delete(files, internalFile)
	entry.Owned = slices.DeleteFunc(slices.Clone(entry.Owned), func(p string) bool { return p == internalFile })
	regen := regenerate(t, files, entry)
	if len(regen.Owned) != 1 || regen.Owned[0].Path != internalFile || !bytes.Contains(regen.Owned[0].Content, []byte("func TestPlanOf(")) {
		t.Errorf("owned = %+v", regen.Owned)
	}
	if !slices.Contains(regen.Entry.Owned, internalFile) || !slices.IsSorted(regen.Entry.Owned) {
		t.Errorf("entry owned = %v", regen.Entry.Owned)
	}
}

func TestRegenerateNamesAGoneMainFile(t *testing.T) {
	t.Parallel()
	out := newAdapter(t, gentest.Contracts(), "Plans")
	files := written(gentest.Contracts(), out)
	delete(files, mainFile)
	regen := regenerate(t, files, out.Entry)
	want := []genkind.Decision{{Path: mainFile, Line: 1, Text: mainFile + " is gone: restore it, golaw never rewrites owned code"}}
	if len(regen.Appends) != 0 || !slices.Equal(regen.Decisions, want) {
		t.Errorf("appends %+v, decisions %+v", regen.Appends, regen.Decisions)
	}
	if len(regen.Bindings) != 1 {
		t.Errorf("the first adapter of the port lost its binding: %+v", regen.Bindings)
	}
}

func TestRegenerateRefusesAnEntryWithoutContract(t *testing.T) {
	t.Parallel()
	entry := manifest.Entry{Kind: genkind.KindAdapterSQL, Template: "adapter-sql@1", Name: "postgres.Plans"}
	_, err := adaptersql.Kind().Regenerate(t.Context(), input(t, gentest.Contracts(), portEntry()), entry)
	if !errors.Is(err, genkind.ErrRequest) {
		t.Errorf("error = %v, want ErrRequest", err)
	}
	broken := gentest.Contracts()
	broken[portsPath] = bytes.Replace(broken[portsPath], []byte(") error\n"), []byte(") bool\n"), 1)
	plans := manifest.Entry{Kind: genkind.KindAdapterSQL, Template: "adapter-sql@1", Name: "postgres.Plans", Contracts: portEntry().Contracts}
	if _, err := adaptersql.Kind().Regenerate(t.Context(), input(t, broken, portEntry(), plans), plans); !errors.Is(err, genkind.ErrRequest) {
		t.Errorf("error = %v, want ErrRequest", err)
	}
}

// unjudged answers every sqlc output as a file the reader cannot judge.
type unjudged struct{ contract.Reader }

func (unjudged) Queries(dir string) ([]genkind.Query, error) {
	return nil, fmt.Errorf("%w: %s", genkind.ErrCannotJudge, dir)
}

func TestNewKeepsTheCannotJudgeOfTheSQLCOutput(t *testing.T) {
	t.Parallel()
	in := input(t, gentest.Contracts(), portEntry())
	in.Contracts = unjudged{contract.Reader{Snapshot: in.Snapshot}}
	_, err := adaptersql.Kind().New(t.Context(), in, request("Plans"))
	if !errors.Is(err, genkind.ErrCannotJudge) || errors.Is(err, genkind.ErrRequest) {
		t.Errorf("error = %v, want ErrCannotJudge alone", err)
	}
}

func TestNewRefusesAPackageThatDoesNotParse(t *testing.T) {
	t.Parallel()
	files := gentest.Contracts()
	files[dir+"/broken.go"] = []byte("package postgres\n\nfunc {\n")
	_, err := adaptersql.Kind().New(t.Context(), input(t, files, portEntry()), request("Plans"))
	if !errors.Is(err, genkind.ErrCannotJudge) {
		t.Errorf("error = %v, want ErrCannotJudge", err)
	}
}

func TestNewNamesTheSupportPackageItCalls(t *testing.T) {
	t.Parallel()
	in := input(t, gentest.Contracts(), portEntry())
	in.Policy.Support = "internal/shared"
	out, err := adaptersql.Kind().New(t.Context(), in, request("Plans"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(byPath(out.Owned)[mainFile], []byte(`gensupport "`+sliceModule+`/internal/shared"`)) {
		t.Errorf("%s does not name its support import gensupport:\n%s", mainFile, byPath(out.Owned)[mainFile])
	}
}

// TestNewWritesEveryConstraintForm holds the four forms of a method's hole:
// the shared mapper, the affected rows, a plain conversion and a missing query.
func TestNewWritesEveryConstraintForm(t *testing.T) {
	t.Parallel()
	files := gentest.Contracts()
	files[portsPath] = bytes.Replace(files[portsPath], []byte("\tArchivePlan(ctx context.Context, account AccountID) error\n"), []byte(
		"\tArchivePlan(ctx context.Context, account AccountID) error\n"+
			"\t// ListPlans lists the plans of one account.\n\tListPlans(ctx context.Context, account AccountID) ([]Plan, error)\n"+
			"\t// CountPlans counts the plans of one account.\n\tCountPlans(ctx context.Context, account AccountID) (int64, error)\n"+
			"\t// PlanNames names the plans of one account.\n\tPlanNames(ctx context.Context, account AccountID) (map[string]Plan, error)\n"), 1)
	sqlc := dir + "/sqlcgen/plans.sql.go"
	files[sqlc] = append(files[sqlc], []byte("\nfunc (q *Queries) ListPlans(ctx context.Context, account string) ([]Plan, error) {\n\treturn nil, nil\n}\n"+
		"\nfunc (q *Queries) CountPlans(ctx context.Context, account string) (int64, error) {\n\treturn 0, nil\n}\n"+
		"\nfunc (q *Queries) PlanNames(ctx context.Context, account string) (Plan, error) {\n\treturn Plan{}, nil\n}\n")...)
	got := openHoles(t, byPath(newAdapter(t, files, "Plans").Owned))
	convert := "convert its answer to the port's types"
	for id, want := range map[string]string{
		"plans.list-plans": "answer planOf of its row and error", "plans.count-plans": convert, "plans.plan-names": convert,
	} {
		if !strings.Contains(got[id], want) {
			t.Errorf("%s = %q, want %q", id, got[id], want)
		}
	}
	mappers := 0
	for id := range got {
		if strings.HasSuffix(id, "-of") {
			mappers++
		}
	}
	if mappers != 1 {
		t.Errorf("holes %v: one shared mapper expected", got)
	}
}

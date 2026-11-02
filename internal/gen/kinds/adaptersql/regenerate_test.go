package adaptersql_test

import (
	"bytes"
	"errors"
	"go/parser"
	"go/token"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/kinds/adaptersql"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/gentest"
)

func TestNewObeysTheTemplateLaws(t *testing.T) {
	t.Parallel()
	first := newAdapter(t, gentest.Contracts(), "Plans")
	if again := newAdapter(t, gentest.Contracts(), "Plans"); !reflect.DeepEqual(first, again) {
		t.Error("two renderings of one contract differ")
	}
	for p, content := range written(map[string][]byte{}, first) {
		if _, err := parser.ParseFile(token.NewFileSet(), p, content, parser.ParseComments); err != nil {
			t.Errorf("%s does not parse: %v", p, err)
		}
		for _, banned := range []string{"http:" + "//", "nolint", "NOSONAR", `"regexp"`, "context.Background()", "context.TO" + "DO()", "\r"} {
			if bytes.Contains(content, []byte(banned)) {
				t.Errorf("%s holds %q", p, banned)
			}
		}
		if lines := bytes.Count(content, []byte("\n")); lines > 300 {
			t.Errorf("%s has %d lines", p, lines)
		}
	}
}

func TestNewRefusesWithTheFix(t *testing.T) {
	t.Parallel()
	noOutput := gentest.Contracts()
	for _, name := range []string{"db.go", "models.go", "plans.sql.go"} {
		delete(noOutput, dir+"/sqlcgen/"+name)
	}
	noError := gentest.Contracts()
	noError[portsPath] = bytes.Replace(noError[portsPath], []byte("ArchivePlan(ctx context.Context, account AccountID) error"), []byte("ArchivePlan(ctx context.Context, account AccountID) bool"), 1)
	existing := manifest.Entry{Kind: genkind.KindAdapterSQL, Template: "adapter-sql@1", Name: "postgres.Plans", Contracts: portEntry().Contracts, Regenerable: []string{dir + "/zz_generated_plans.go"}}
	tests := []struct {
		name    string
		files   map[string][]byte
		entries []manifest.Entry
		err     error
		text    string
	}{
		{"no sqlc output", noOutput, nil, genkind.ErrRequest, "run golaw gen"},
		{"a port method without an error", noError, nil, genkind.ErrRequest, "ArchivePlan"},
		{"an existing instance", gentest.Contracts(), []manifest.Entry{existing}, genkind.ErrExists, dir + "/zz_generated_plans.go"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			in := input(t, tc.files, append([]manifest.Entry{portEntry()}, tc.entries...)...)
			_, err := adaptersql.Kind().New(t.Context(), in, request("Plans"))
			if !errors.Is(err, tc.err) || !strings.Contains(fmtErr(err), tc.text) {
				t.Errorf("error = %v, want %v naming %q", err, tc.err, tc.text)
			}
		})
	}
}

func fmtErr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func TestASecondInstanceCallsTheSharedMapping(t *testing.T) {
	t.Parallel()
	plans := newAdapter(t, gentest.Contracts(), "Plans")
	replica := newAdapter(t, written(gentest.Contracts(), plans), "Replica", plans.Entry)
	owned := byPath(replica.Owned)
	for _, p := range []string{dir + "/doc.go", dir + "/replica_internal_test.go"} {
		if _, ok := owned[p]; ok {
			t.Errorf("the second instance rewrote %s", p)
		}
	}
	got := openHoles(t, owned)
	if _, ok := got["replica.plan-of"]; ok || !strings.Contains(got["replica.read-plan"], "answer planOf of its row and error") {
		t.Errorf("holes = %v", got)
	}
	if len(replica.Bindings) != 0 {
		t.Errorf("the second adapter of the port bound itself: %+v", replica.Bindings)
	}
}

func TestNewNamesTheQueryAMethodLacks(t *testing.T) {
	t.Parallel()
	files := gentest.Contracts()
	files[portsPath] = bytes.Replace(files[portsPath], []byte("\tArchivePlan(ctx context.Context, account AccountID) error\n"),
		[]byte("\tArchivePlan(ctx context.Context, account AccountID) error\n\t// PurgePlans purges the plans of one account.\n\tPurgePlans(ctx context.Context, account AccountID) error\n"), 1)
	got := openHoles(t, byPath(newAdapter(t, files, "Plans").Owned))
	want := "add the sqlc query PurgePlans (-- name: PurgePlans) and run golaw gen, then call p.queries.PurgePlans; wrap the error with gensupport.Prefix(actionPlansPurgePlans)"
	if got["plans.purge-plans"] != want {
		t.Errorf("constraint = %q, want %q", got["plans.purge-plans"], want)
	}
}

func TestRegenerateAppendsTheNewMethodAndNamesTheOrphan(t *testing.T) {
	t.Parallel()
	out := newAdapter(t, gentest.Contracts(), "Plans")
	files := written(gentest.Contracts(), out)
	before := files[dir+"/plans.go"]
	files[dir+"/plans.go"] = append(bytes.Clone(before), []byte("\n// PurgeAll purges every plan.\nfunc (p *Plans) PurgeAll() error {\n\treturn nil\n}\n")...)
	files[portsPath] = bytes.Replace(files[portsPath], []byte("\tArchivePlan(ctx context.Context, account AccountID) error\n"),
		[]byte("\tArchivePlan(ctx context.Context, account AccountID) error\n\t// PurgePlans purges the plans of one account.\n\tPurgePlans(ctx context.Context, account AccountID) error\n"), 1)
	regen, err := adaptersql.Kind().Regenerate(t.Context(), input(t, files, portEntry(), out.Entry), out.Entry)
	if err != nil {
		t.Fatal(err)
	}
	appends := make(map[string]genkind.Append)
	for _, a := range regen.Appends {
		appends[a.Path] = a
	}
	if a := appends[dir+"/plans.go"]; len(a.Decls) != 1 || !slices.Equal(a.Holes, []string{"plans.purge-plans"}) {
		t.Errorf("plans.go append = %+v", a)
	}
	if a := appends[dir+"/plans_test.go"]; len(a.Decls) != 1 || !bytes.Contains(a.Decls[0], []byte("func TestPlansPurgePlans(t *testing.T)")) {
		t.Errorf("plans_test.go append = %+v", a)
	}
	line := bytes.Count(before, []byte("\n")) + 3
	want := []genkind.Decision{{
		Path: dir + "/plans.go", Line: line,
		Text: "(*Plans).PurgeAll is no longer in port Ledger: delete it or restore the method in internal/application/growth/ports.go",
	}}
	if !slices.Equal(regen.Decisions, want) {
		t.Errorf("decisions = %+v, want %+v", regen.Decisions, want)
	}
	if !bytes.Contains(byPath(regen.Regenerable)[dir+"/zz_generated_plans.go"], []byte(`actionPlansPurgePlans  = "postgres: plans purge plans"`)) {
		t.Error("the regenerable file lacks the new action constant")
	}
	if len(regen.Owned) != 0 {
		t.Errorf("Regenerate wrote owned files %+v", regen.Owned)
	}
}

func TestRegenerateOfAnUnchangedContractChangesNothing(t *testing.T) {
	t.Parallel()
	out := newAdapter(t, gentest.Contracts(), "Plans")
	files := written(gentest.Contracts(), out)
	regen, err := adaptersql.Kind().Regenerate(t.Context(), input(t, files, portEntry(), out.Entry), out.Entry)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(regen.Regenerable, out.Regenerable) || len(regen.Appends) != 0 || len(regen.Decisions) != 0 || !reflect.DeepEqual(regen.Entry, out.Entry) {
		t.Errorf("regenerate of an unchanged contract = %+v", regen)
	}
}

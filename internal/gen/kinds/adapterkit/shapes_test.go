package adapterkit_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/kinds/adapterkit"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/gen/render"
	"github.com/cristianargotti/golaw/internal/gentest"
)

func TestMethodsQualifyNestedShapesAndRefuseTheRest(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, methods, signature string
		err                      error
		text                     string
	}{
		{
			"func and chan", "\tRun(ctx context.Context, fn func(Plan) (AccountID, error), ch chan<- AccountID) error\n",
			"(ctx context.Context, fn func(growth.Plan) (growth.AccountID, error), ch chan<- growth.AccountID) error", nil, "",
		},
		{
			"near: a named array length and a variadic", "\tRun(ctx context.Context, buf [Size]byte, ids ...AccountID) error\n",
			"(ctx context.Context, buf [growth.Size]byte, ids ...growth.AccountID) error", nil, "",
		},
		{
			"near: a literal array length", "\tRun(ctx context.Context, buf [4]Plan) error\n",
			"(ctx context.Context, buf [4]growth.Plan) error", nil, "",
		},
		{"an inline struct", "\tRun(ctx context.Context, q struct{ ID AccountID }) error\n", "", genkind.ErrCannotJudge, "is written inline"},
		{"a generic instantiation", "\tRun(ctx context.Context, q Page[Plan]) error\n", "", genkind.ErrCannotJudge, "Page[Plan]"},
		{"a package the file does not import", "\tRun(ctx context.Context, q sql.NullString) error\n", "", genkind.ErrCannotJudge, "sql"},
		{"an unexported result", "\tRun(ctx context.Context) (plan, error)\n", "", genkind.ErrCannotJudge, "plan"},
		{"an unexported array length", "\tRun(ctx context.Context, buf [size]byte) error\n", "", genkind.ErrCannotJudge, "size"},
		{"an unexported func parameter", "\tRun(ctx context.Context, fn func(plan)) error\n", "", genkind.ErrCannotJudge, "plan"},
		{"an unexported map key", "\tRun(ctx context.Context, m map[plan]int) error\n", "", genkind.ErrCannotJudge, "plan"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			methods, _, err := adapterkit.Methods(plansTarget(t, portsWith(tc.methods)))
			if !errors.Is(err, tc.err) || (tc.err == nil) != (err == nil) {
				t.Fatalf("error = %v, want %v", err, tc.err)
			}
			if err != nil {
				if !strings.Contains(err.Error(), tc.text) {
					t.Errorf("error %q does not name %q", err, tc.text)
				}
				return
			}
			if got := methods[0].Signature(); got != tc.signature {
				t.Errorf("Signature = %q, want %q", got, tc.signature)
			}
		})
	}
}

func TestMethodsSettleEveryNameATemplateWrites(t *testing.T) {
	t.Parallel()
	target := plansTarget(t, portsWith("\tWrite(ctx context.Context, t Plan, err AccountID, zero int) (Plan, int, error)\n"+
		"\t// writes nothing back\n\tTouch(ctx context.Context, p AccountID) error\n"))
	if target.Recv != "plans" {
		t.Errorf("Recv = %q, want the whole name beside a parameter p", target.Recv)
	}
	methods, imports, err := adapterkit.Methods(target)
	if err != nil {
		t.Fatal(err)
	}
	write, touch := methods[0], methods[1]
	wantZeros := []genkind.Param{{Name: "zero2", Type: "growth.Plan"}, {Name: "zero3", Type: "int"}}
	wantInputs := []genkind.Param{{Name: "tValue", Type: "growth.Plan"}, {Name: "errValue", Type: "growth.AccountID"}, {Name: "zero", Type: "int"}}
	if !slices.Equal(write.Zeros, wantZeros) || !slices.Equal(write.Inputs, wantInputs) {
		t.Errorf("zeros %+v, inputs %+v", write.Zeros, write.Inputs)
	}
	tests := map[string][2]string{
		"returns":   {write.Returns(), "zero2, zero3, "},
		"discard":   {write.Discard(), "_, _, "},
		"call":      {write.CallArgs(), "t.Context(), tValue, errValue, zero"},
		"signature": {write.Signature(), "(ctx context.Context, t growth.Plan, err growth.AccountID, zero int) (growth.Plan, int, error)"},
		"write doc": {strings.Join(write.Doc, "|"), "Write implements the Ledger port."},
		"touch doc": {strings.Join(touch.Doc, "|"), "Touch implements the Ledger port.|writes nothing back"},
	}
	for name, pair := range tests {
		if pair[0] != pair[1] {
			t.Errorf("%s = %q, want %q", name, pair[0], pair[1])
		}
	}
	if len(imports) != 2 {
		t.Errorf("imports = %+v", imports)
	}
}

func TestMethodsOfAPortOfPredeclaredTypesImportNoPortPackage(t *testing.T) {
	t.Parallel()
	_, imports, err := adapterkit.Methods(plansTarget(t, portsWith("\tPing(ctx context.Context, n int) error\n")))
	if err != nil || !slices.Equal(imports, []genkind.Import{{Path: "context"}}) {
		t.Errorf("imports = %+v, %v", imports, err)
	}
}

func TestFromEntryAndTheRefusalsOfAPort(t *testing.T) {
	t.Parallel()
	in := input(t, gentest.Contracts(), portEntry())
	layout := in.Policy.Layout.AdapterHTTP
	entry := manifest.Entry{Kind: genkind.KindAdapterHTTP, Template: "adapter-http@1", Name: "planapi.Client", Contracts: portEntry().Contracts}
	got, err := adapterkit.FromEntry(in, layout, entry)
	if err != nil || got.Dir != "internal/adapters/out/planapi" || got.Recv != "c" {
		t.Errorf("FromEntry = %+v, %v", got, err)
	}
	entry.Contracts = nil
	if _, err := adapterkit.FromEntry(in, layout, entry); !errors.Is(err, genkind.ErrRequest) {
		t.Errorf("an entry without contracts = %v", err)
	}
	bare := portEntry()
	bare.Contracts = nil
	req := adapterRequest("postgres", "Plans", "growth.Ledger")
	if _, err := adapterkit.FromRequest(input(t, gentest.Contracts(), bare), genkind.KindAdapterSQL, layout, req); !errors.Is(err, genkind.ErrRequest) ||
		!strings.Contains(err.Error(), "golaw gen") {
		t.Errorf("a port without contracts = %v", err)
	}
	missing := portEntry()
	missing.Contracts = []manifest.Contract{{Path: portsPath, Symbol: "Absent"}}
	if _, err := adapterkit.FromRequest(input(t, gentest.Contracts(), missing), genkind.KindAdapterSQL, layout, req); err == nil {
		t.Error("a symbol the contract lacks resolved")
	}
}

func TestAnExistingInstanceNamesItsFile(t *testing.T) {
	t.Parallel()
	owned := manifest.Entry{
		Kind: genkind.KindAdapterSQL, Template: "adapter-sql@1", Name: "postgres.Plans",
		Owned: []string{"internal/adapters/out/postgres/plans.go"},
	}
	bare := owned
	bare.Owned = nil
	for want, existing := range map[string]manifest.Entry{owned.Owned[0]: owned, manifest.Path: bare} {
		in := input(t, gentest.Contracts(), portEntry(), existing)
		_, err := adapterkit.FromRequest(in, genkind.KindAdapterSQL, in.Policy.Layout.AdapterSQL, adapterRequest("postgres", "Plans", "growth.Ledger"))
		if !errors.Is(err, genkind.ErrExists) || !strings.Contains(err.Error(), want) {
			t.Errorf("error = %v, want it to name %s", err, want)
		}
	}
}

func TestUsesNamesAVersionedPathByItsElementBefore(t *testing.T) {
	t.Parallel()
	imports := []genkind.Import{{Path: "example.com/lib/v2"}, {Path: "example.com/v"}, {Path: "example.com/vx"}, {Path: "example.com/lib/v2"}}
	got := adapterkit.Uses(imports, []string{"lib.Item", "v.Thing", "vx.Other"})
	want := []genkind.Import{{Path: "example.com/lib/v2"}, {Path: "example.com/v"}, {Path: "example.com/vx"}}
	if !slices.Equal(got, want) {
		t.Errorf("Uses = %+v, want %+v", got, want)
	}
	if got := adapterkit.Uses(imports, []string{"élib.Item", "x.lib.Item"}); len(got) != 0 {
		t.Errorf("a longer name selected an import: %+v", got)
	}
}

func TestScanAndSentinelsRefuseAFileThatDoesNotParse(t *testing.T) {
	t.Parallel()
	files := gentest.Contracts()
	files["internal/adapters/out/postgres/plans.go"] = []byte("package postgres\n\nfunc (p Plans) Value() {}\n\nfunc (o Other) Skip() {}\n\nfunc (Plans) Blank() {}\n")
	files["internal/adapters/out/postgres/README.md"] = []byte("not go\n")
	target := plansTarget(t, files)
	pkg, err := adapterkit.Scan(gentest.Snapshot(t, files), target.Dir, "Plans")
	if err != nil || len(pkg.Methods) != 2 || pkg.Doc {
		t.Errorf("Scan = %+v, %v", pkg, err)
	}
	files["internal/adapters/out/postgres/broken.go"] = []byte("package postgres\n\nfunc (\n")
	files["internal/application/growth/broken.go"] = []byte("package growth\n\nvar (\n")
	snap := gentest.Snapshot(t, files)
	if _, err := adapterkit.Scan(snap, target.Dir, "Plans"); !errors.Is(err, genkind.ErrCannotJudge) {
		t.Errorf("Scan of a broken file = %v", err)
	}
	if _, err := adapterkit.Sentinels(snap, target.Port, "NotFound"); !errors.Is(err, genkind.ErrCannotJudge) {
		t.Errorf("Sentinels of a broken file = %v", err)
	}
}

func TestBindingsIgnoreOtherKindsAndOtherPorts(t *testing.T) {
	t.Parallel()
	b := genkind.Binding{Kind: genkind.KindAdapterSQL, Port: "Ledger"}
	clock := manifest.Entry{
		Kind: genkind.KindAdapterHTTP, Template: "adapter-http@1", Name: "clockapi.Client",
		Contracts: []manifest.Contract{{Path: portsPath, Symbol: "Clock"}},
	}
	sqlEntry := manifest.Entry{Kind: genkind.KindAdapterSQL, Template: "adapter-sql@1", Name: "postgres.Plans", Contracts: portEntry().Contracts}
	base := manifest.Manifest{Schema: manifest.Schema}.With(portEntry()).With(clock)
	if got := adapterkit.Bindings(base, sqlEntry, b); len(got) != 1 {
		t.Errorf("an adapter of another port took the binding: %+v", got)
	}
	if got := adapterkit.Bindings(base, portEntry(), b); got != nil {
		t.Errorf("a port entry bound: %+v", got)
	}
	sqlEntry.Contracts = nil
	if got := adapterkit.Bindings(base, sqlEntry, b); got != nil {
		t.Errorf("an entry without contracts bound: %+v", got)
	}
}

func TestDeclNamesTheTemplateItCannotRead(t *testing.T) {
	t.Parallel()
	_, err := adapterkit.Decl(fstest.MapFS{}, "absent.go.tmpl", nil)
	if !errors.Is(err, render.ErrTemplate) || !strings.Contains(err.Error(), "absent.go.tmpl") {
		t.Errorf("Decl = %v", err)
	}
}

func TestAPackageClauseUnlikeItsDirectoryNamesTheImport(t *testing.T) {
	t.Parallel()
	files := gentest.Contracts()
	files[portsPath] = []byte("// Package ledger is declared in the growth directory.\npackage ledger\n\nimport \"context\"\n\n" +
		"// Ledger stores the plans of an account.\ntype Ledger interface {\n\tReadPlan(ctx context.Context, account AccountID) (Plan, error)\n}\n")
	target := plansTarget(t, files)
	if target.PortPkg != (genkind.Import{Name: "ledger", Path: sliceModule + "/internal/application/growth"}) {
		t.Errorf("PortPkg = %+v", target.PortPkg)
	}
	methods, imports, err := adapterkit.Methods(target)
	if err != nil || methods[0].Signature() != "(ctx context.Context, account ledger.AccountID) (ledger.Plan, error)" ||
		!slices.Contains(imports, target.PortPkg) {
		t.Errorf("Methods = %+v, %+v, %v", methods, imports, err)
	}
}

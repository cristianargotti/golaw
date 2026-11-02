package port

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/goldentest"
	"github.com/cristianargotti/golaw/internal/genheader"
)

const ledgerTest = "internal/application/growth/ledgertest/"

func TestTheRegenerablePartIsThreeFilesWithTheGolawHeader(t *testing.T) {
	t.Parallel()
	files := rendered(t, ledgerPort())
	names := make([]string, 0, len(files))
	for path, content := range files {
		names = append(names, path)
		h, ok := genheader.Parse([]byte(content))
		if !ok || h != (genheader.Header{Version: "0.3.0", Kind: genkind.KindPort, Template: 1}) {
			t.Errorf("%s header = %+v, %v", path, h, ok)
		}
		if _, err := parser.ParseFile(token.NewFileSet(), path, content, parser.ParseComments); err != nil {
			t.Errorf("%s does not parse: %v", path, err)
		}
	}
	slices.Sort(names)
	want := []string{ledgerTest + "zz_generated_fake.go", ledgerTest + "zz_generated_suite.go", ledgerTest + "zz_generated_suite_test.go"}
	if !slices.Equal(names, want) {
		t.Errorf("files = %v, want %v", names, want)
	}
	again := rendered(t, ledgerPort())
	for path, content := range files {
		if again[path] != content {
			t.Errorf("%s differs between two renderings", path)
		}
	}
}

func TestTheFakeOfLedgerRecordsAndAnswers(t *testing.T) {
	t.Parallel()
	fake := rendered(t, ledgerPort())[ledgerTest+"zz_generated_fake.go"]
	assertHas(t, fake,
		"package ledgertest",
		`MethodReadPlan = "ReadPlan"`,
		`MethodArchivePlan = "ArchivePlan"`,
		"var _ growth.Ledger = (*Fake)(nil)",
		"type Fake struct { rec *gensupport.Recorder }",
		"func NewFake() *Fake { return &Fake{rec: gensupport.NewRecorder()} }",
		"func (f *Fake) ReadPlan(ctx context.Context, account growth.AccountID) (growth.Plan, error) {",
		"r, err := porttest.Answer(ctx, f.rec, MethodReadPlan, account) return porttest.Next[growth.Plan](r), err",
		"func (f *Fake) ArchivePlan(ctx context.Context, account growth.AccountID) error {",
		"_, err := porttest.Answer(ctx, f.rec, MethodArchivePlan, account) return err",
		"func (f *Fake) Program(method string, results ...any) { f.rec.Program(method, results...) }",
		"func (f *Fake) Calls(method string) [][]any { return f.rec.Calls(method) }",
	)
	want := []string{"context", module + "/internal/application/growth", module + "/internal/gensupport", module + "/internal/gensupport/porttest"}
	if got := importPaths(t, fake); !slices.Equal(got, want) {
		t.Errorf("fake imports %v, want %v", got, want)
	}
}

func TestTheSuiteOfLedgerCallsThePorttestPair(t *testing.T) {
	t.Parallel()
	suite := rendered(t, ledgerPort())[ledgerTest+"zz_generated_suite.go"]
	assertHas(t, suite,
		"type Case = porttest.Case",
		"type Subject = porttest.Subject[growth.Ledger]",
		"type Factory = porttest.Factory[growth.Ledger]",
		"func Run(t *testing.T, factory Factory) { t.Helper() porttest.Run(t, factory, methods(), Cases()) }",
		"func Cases() []Case { return slices.Concat(readPlanCases(), archivePlanCases()) }",
		`{Name: MethodReadPlan, Hole: "ledger.read-plan", Context: true, Failing: true, Call: callReadPlan},`,
		`{Name: MethodArchivePlan, Hole: "ledger.archive-plan", Context: true, Failing: true, Call: callArchivePlan},`,
		"func callReadPlan(ctx context.Context, p growth.Ledger, args []any) []any {",
		"in := porttest.NewValues(args) return porttest.Pack(p.ReadPlan(ctx, porttest.Next[growth.AccountID](in)))",
	)
	want := []string{"context", "slices", "testing", module + "/internal/application/growth", module + "/internal/gensupport/porttest"}
	if got := importPaths(t, suite); !slices.Equal(got, want) {
		t.Errorf("suite imports %v, want %v", got, want)
	}
	test := rendered(t, ledgerPort())[ledgerTest+"zz_generated_suite_test.go"]
	assertHas(t, test,
		"package ledgertest_test",
		"func TestFakeKeepsTheContractOfLedger(t *testing.T) {",
		"fake.Program(c.Method, append(slices.Clone(c.Want), c.WantErr)...)",
		"func TestFakeRecordsWhatItIsCalledWith(t *testing.T) {",
		"fake.Program(ledgertest.MethodReadPlan, errors.ErrUnsupported)",
		"var account growth.AccountID _, err := fake.ReadPlan(t.Context(), account)",
		"if !errors.Is(err, errors.ErrUnsupported) {",
		"if got := fake.Calls(ledgertest.MethodReadPlan); len(got) != 1 {",
	)
}

func TestEveryBodyShapeOfTheFake(t *testing.T) {
	t.Parallel()
	files := rendered(t, shapesPort())
	assertHas(t, files[ledgerTest+"zz_generated_fake.go"],
		"func (f *Fake) Forget(_ context.Context, account growth.AccountID) { porttest.Record(f.rec, MethodForget, account) }",
		"func (f *Fake) Validate(plan growth.Plan) error { _, err := porttest.Failing(f.rec, MethodValidate, plan) return err }",
		"func (f *Fake) Save(ctx context.Context, plans ...growth.Plan) error { _, err := porttest.Answer(ctx, f.rec, MethodSave, plans) return err }",
	)
	assertHas(t, files[ledgerTest+"zz_generated_suite.go"],
		`{Name: MethodForget, Hole: "ledger.forget", Context: true, Failing: false, Call: callForget},`,
		"p.Forget(ctx, porttest.Next[growth.AccountID](in)) return nil",
		"func callValidate(_ context.Context, p growth.Ledger, args []any) []any {",
		"return porttest.Pack(p.Validate(porttest.Next[growth.Plan](in)))",
		"return porttest.Pack(p.Save(ctx, porttest.Next[[]growth.Plan](in)...))",
		"return slices.Concat(forgetCases(), validateCases(), saveCases())",
	)
	assertHas(t, files[ledgerTest+"zz_generated_suite_test.go"], "var account growth.AccountID fake.Forget(t.Context(), account)")
	if strings.Contains(files[ledgerTest+"zz_generated_suite_test.go"], "errors.Is") {
		t.Error("the recording test checks an error Forget does not answer")
	}
}

func TestTheClockFakeHasNoContextAndNoError(t *testing.T) {
	t.Parallel()
	files := rendered(t, clockPort())
	dir := "internal/application/growth/clocktest/"
	fake := files[dir+"zz_generated_fake.go"]
	assertHas(t, fake, "package clocktest", "func (f *Fake) Now() time.Time { r := porttest.Record(f.rec, MethodNow) return porttest.Next[time.Time](r) }")
	want := []string{"time", module + "/internal/application/growth", module + "/internal/gensupport", module + "/internal/gensupport/porttest"}
	if got := importPaths(t, fake); !slices.Equal(got, want) {
		t.Errorf("clock fake imports %v, want %v", got, want)
	}
	assertHas(t, files[dir+"zz_generated_suite.go"],
		"func Cases() []Case { return nowCases() }",
		"func callNow(_ context.Context, p growth.Clock, _ []any) []any { return porttest.Pack(p.Now()) }",
	)
	assertHas(t, files[dir+"zz_generated_suite_test.go"], "fake.Now()")
}

func TestAnInterfaceStubKeepsItsHoleRed(t *testing.T) {
	t.Parallel()
	files := rendered(t, stubPort())
	fake, suite, test := files[ledgerTest+"zz_generated_fake.go"], files[ledgerTest+"zz_generated_suite.go"], files[ledgerTest+"zz_generated_suite_test.go"]
	if strings.Contains(fake, "const (") || strings.Contains(fake, "porttest") {
		t.Errorf("the fake of a port without methods names methods or the suite:\n%s", fake)
	}
	assertHas(t, suite,
		`porttest.Open(t, "ledger.methods", "declare the methods of Ledger in internal/application/growth/ports.go: each takes a context.Context first and answers an error last, at most 7 parameters")`,
		"func Cases() []Case { return nil }",
		"func methods() []porttest.Method[growth.Ledger] { return nil }",
	)
	if strings.Contains(test, "TestFakeRecordsWhatItIsCalledWith") {
		t.Error("the suite test of a port without methods calls a method")
	}
}

func TestTheRegenerablePartViolatesNoTemplateLaw(t *testing.T) {
	t.Parallel()
	for _, p := range []genkind.Port{ledgerPort(), clockPort(), shapesPort(), stubPort(), limitPort()} {
		files := map[string][]byte{}
		for path, content := range rendered(t, p) {
			files[path] = []byte(content)
		}
		for _, law := range goldentest.Laws() {
			for _, violation := range law.Assert(files) {
				t.Errorf("%s %s: %s", p.Name, law.Name, violation)
			}
		}
	}
}

func TestAPortAtTheLimitsStaysWithinTheSizeLaws(t *testing.T) {
	t.Parallel()
	for path, content := range rendered(t, limitPort()) {
		if lines := strings.Count(content, "\n"); lines > 300 {
			t.Errorf("%s holds %d lines, over 300", path, lines)
		}
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		assertFunctionsWithin40(t, path, content)
	}
}

// assertFunctionsWithin40 holds every function body of one file to the
// 40 lines of the size law.
func assertFunctionsWithin40(t *testing.T, path, content string) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, content, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range f.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil {
			if n := fset.Position(fn.Body.Rbrace).Line - fset.Position(fn.Body.Lbrace).Line - 1; n > 40 {
				t.Errorf("%s %s spans %d lines, over 40", path, fn.Name.Name, n)
			}
		}
	}
}

func TestSupportImportNamesAMovedSupportPackageGensupport(t *testing.T) {
	t.Parallel()
	in := input()
	in.Policy.Support = "internal/platform/support"
	tests := map[string]struct {
		sub  string
		want genkind.Import
	}{
		"red: the moved root keeps the name the templates call": {want: genkind.Import{Name: "gensupport", Path: module + "/internal/platform/support"}},
		"near: a subpackage keeps its own name":                 {sub: "/porttest", want: genkind.Import{Path: module + "/internal/platform/support/porttest"}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := supportImport(in, tc.sub); got != tc.want {
				t.Errorf("supportImport(%q) = %+v, want %+v", tc.sub, got, tc.want)
			}
		})
	}
	if got := supportImport(input(), ""); got != (genkind.Import{Path: module + "/internal/gensupport"}) {
		t.Errorf("supportImport of the default root = %+v, want no explicit name", got)
	}
}

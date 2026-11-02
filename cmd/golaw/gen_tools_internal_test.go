package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen"
	"github.com/cristianargotti/golaw/internal/gen/apply"
	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/holes"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/gen/thirdparty"
	"github.com/cristianargotti/golaw/internal/runnertest"
)

// errNoRepository is the discovery failure a test injects.
var errNoRepository = errors.New("no repository")

// toolsBinding is a binding whose PATH holds only bin.
func toolsBinding(t *testing.T, bin string) binding {
	t.Helper()
	b, _ := testBinding(t)
	b.environ = func() []string { return []string{"HOME=/nowhere", "PATH=" + bin} }
	return b
}

func TestGoBinaryReadsTheBindingPath(t *testing.T) {
	t.Parallel()
	bin := t.TempDir()
	goPath := runnertest.Install(t, bin, "go", runnertest.Behavior{})
	if got, err := toolsBinding(t, bin).goBinary(); err != nil || got != goPath {
		t.Errorf("goBinary = %q, %v, want %q", got, err, goPath)
	}
	empty := toolsBinding(t, string(filepath.ListSeparator)+t.TempDir())
	if _, err := empty.goBinary(); !errors.Is(err, thirdparty.ErrMissing) {
		t.Errorf("a PATH without go = %v, want ErrMissing", err)
	}
}

func TestPathOfAnswersTheLastPathInAnyCase(t *testing.T) {
	t.Parallel()
	if got := pathOf([]string{"PATH=/a", "HOME=/h", "Path=/b", "broken"}); got != "/b" {
		t.Errorf("pathOf = %q", got)
	}
	if got := pathOf(nil); got != "" {
		t.Errorf("pathOf(nil) = %q", got)
	}
}

func TestAutofixRunsTheSafeFixersOverTheWrittenPackages(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeRepoFile(t, root, "internal/plans/plans.go", "package plans\n")
	written := []string{"internal/plans/plans.go", "internal/plans/zz_generated_plans.go", "README.md"}
	bin := t.TempDir()
	runnertest.Install(t, bin, "go", runnertest.Behavior{Code: 1})
	b := toolsBinding(t, bin)
	if lines := b.autofix(t.Context(), root, written); lines != nil {
		t.Errorf("without a tools module = %q, want nothing", lines)
	}
	writeRepoFile(t, root, "tools/go.mod", "module example.com/slice/tools\n")
	if lines := b.autofix(t.Context(), root, []string{"README.md"}); lines != nil {
		t.Errorf("without a Go file = %q, want nothing", lines)
	}
	// A linter that exits non-zero left findings for make lint, never a failure.
	if lines := b.autofix(t.Context(), root, written); lines != nil {
		t.Errorf("an unchanged tree = %q, want nothing", lines)
	}
	want := []string{"tool", "-modfile=tools/go.mod", "golangci-lint", "run", "--fix", "--enable-only", safeFixers, "./internal/plans"}
	if calls := runnertest.Calls(t, bin, "go"); len(calls) != 1 || !reflect.DeepEqual(calls[0], want) {
		t.Errorf("go calls = %q, want one %q", calls, want)
	}
	missing := toolsBinding(t, t.TempDir())
	if lines := missing.autofix(t.Context(), root, written); len(lines) != 1 || !strings.HasPrefix(lines[0], "not fixed: golangci-lint did not run: ") {
		t.Errorf("without go = %q", lines)
	}
}

func TestAutofixNamesAGoThatCannotStart(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("an executable bit does not exist on windows")
	}
	root, bin := t.TempDir(), t.TempDir()
	writeRepoFile(t, root, "tools/go.mod", "module example.com/slice/tools\n")
	writeRepoFile(t, root, "internal/plans/plans.go", "package plans\n")
	writeRepoFile(t, bin, "go", "not a program\n")
	if err := os.Chmod(filepath.Join(bin, "go"), 0o700); err != nil {
		t.Fatal(err)
	}
	lines := toolsBinding(t, bin).autofix(t.Context(), root, []string{"internal/plans/plans.go"})
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "not fixed: golangci-lint did not run: ") {
		t.Errorf("lines = %q", lines)
	}
}

func TestFixedLinesNameEachChangedFile(t *testing.T) {
	t.Parallel()
	before := map[string][]byte{"a.go": []byte("a"), "b.go": []byte("b")}
	after := map[string][]byte{"a.go": []byte("a"), "b.go": []byte("b2"), "c.go": []byte("c")}
	want := []string{"fixed b.go: golangci-lint fixers", "fixed c.go: golangci-lint fixers"}
	if got := fixedLines(before, after); !reflect.DeepEqual(got, want) {
		t.Errorf("fixedLines = %q", got)
	}
}

func TestThirdPartyDriftNamesWhatThePinnedGeneratorWritesDifferently(t *testing.T) {
	t.Parallel()
	ta, root, _ := fakeGenGo(t, runnertest.Behavior{Stdout: "ok"})
	run, err := ta.prepareGen(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	gens := thirdparty.Generators(run.policy, run.snapshot)
	drift, unmeasured, err := ta.bind().thirdPartyDrift(t.Context(), root, run.snapshot, gens)
	if err != nil || len(unmeasured) != 0 || len(drift) == 0 {
		t.Fatalf("drift = %+v, unmeasured %q, %v", drift, unmeasured, err)
	}
	for _, d := range drift {
		if d.Cause != gen.DriftThirdParty || d.Fix != gen.FixGen || !strings.HasPrefix(d.Path, "internal/adapters/out/postgres/sqlcgen/") {
			t.Errorf("drift = %+v", d)
		}
	}
	if drift, unmeasured, err := ta.bind().thirdPartyDrift(t.Context(), root, run.snapshot, nil); drift != nil || unmeasured != nil || err != nil {
		t.Errorf("no generator = %+v, %q, %v", drift, unmeasured, err)
	}
}

func TestThirdPartyDriftIsUnmeasuredWithoutGo(t *testing.T) {
	t.Parallel()
	gens := []thirdparty.Generator{{Name: "sqlc"}, {Name: "oapi-codegen"}}
	_, unmeasured, err := toolsBinding(t, t.TempDir()).thirdPartyDrift(t.Context(), t.TempDir(), nil, gens)
	if err != nil || len(unmeasured) != 2 || !strings.HasPrefix(unmeasured[1], "oapi-codegen: ") {
		t.Errorf("unmeasured = %q, %v", unmeasured, err)
	}
}

func TestThirdPartyDriftAnswersAGeneratorThatFails(t *testing.T) {
	t.Parallel()
	ta, _, _ := fakeGenGo(t, runnertest.Behavior{Stderr: "plans.sql:2: syntax error", Code: 1})
	if code := ta.run(t.Context(), []string{"gen", "--check"}); code != exitFailure || !strings.Contains(ta.stderr.String(), "plans.sql:2: syntax error") {
		t.Errorf("exit = %d, stderr %q", code, ta.stderr.String())
	}
}

func TestRunGeneratorsWithoutGoIsMissing(t *testing.T) {
	t.Parallel()
	b := toolsBinding(t, t.TempDir())
	if lines, err := b.runGenerators(t.Context(), t.TempDir(), []thirdparty.Generator{{Name: "sqlc"}}); !errors.Is(err, thirdparty.ErrMissing) || lines != nil {
		t.Errorf("runGenerators = %q, %v", lines, err)
	}
}

func TestGenLinesNameEveryFact(t *testing.T) {
	t.Parallel()
	hole := holes.Hole{Path: "internal/plans/plans.go", Line: 7, ID: "plans.read-plan", Constraint: "reads one plan"}
	res := gen.Result{
		Opened: []holes.Hole{hole}, Closed: []holes.Hole{{Path: "internal/plans/plans.go", ID: "plans.archive-plan"}},
		Decisions: []genkind.Decision{{Path: "golaw.yml", Line: 1, Text: "wire it by hand"}},
		Removed:   []manifest.Entry{{Kind: genkind.KindAdapterSQL, Name: "postgres.Plans"}},
	}
	report := apply.Report{Written: []string{"internal/plans/plans.go"}, Skipped: []apply.Skip{{Path: "x.go", Cause: apply.CauseChanged}}}
	lines := genLines(res, report, []string{"ran sqlc sqlc.yaml"})
	for _, want := range []string{
		"golaw gen: 1 files changed, 1 holes opened, 1 closed, 1 decisions", "written internal/plans/plans.go",
		"skipped x.go: " + apply.CauseChanged, "hole open internal/plans/plans.go:7 plans.read-plan reads one plan",
		"hole closed internal/plans/plans.go plans.archive-plan", "decision golaw.yml:1 wire it by hand", "ran sqlc sqlc.yaml",
	} {
		if !slices.Contains(lines, want) {
			t.Errorf("lines lack %q: %q", want, lines)
		}
	}
	person := genPersonLines(res, report)
	for _, want := range []string{"entrada removida adapter-sql postgres.Plans", "lacuna fechada internal/plans/plans.go plans.archive-plan", "decisão golaw.yml:1 wire it by hand", "arquivo gerado internal/plans/plans.go"} {
		if !slices.Contains(person, want) {
			t.Errorf("person lines lack %q: %q", want, person)
		}
	}
	data, err := json.Marshal(genDocument(res, report, nil))
	if err != nil || !strings.Contains(string(data), `"notices":[]`) || !strings.Contains(string(data), `"removed":[{"kind":"adapter-sql","name":"postgres.Plans"}]`) || !strings.Contains(string(data), `"decisions":[{"path":"golaw.yml","line":1`) {
		t.Errorf("JSON = %s, %v", data, err)
	}
}

func TestCheckPersonLinesSpeakPortuguese(t *testing.T) {
	t.Parallel()
	causes := []string{gen.DriftDiffers, gen.DriftMissing, gen.DriftContractGone, gen.DriftThirdParty, gen.DriftStub, "other"}
	drift := make([]gen.Drift, 0, len(causes))
	for _, cause := range causes {
		drift = append(drift, gen.Drift{Path: "p.go", Cause: cause, Fix: gen.FixGen})
	}
	lines := checkPersonLines(drift, nil, []string{"sqlc: go is not on PATH"})
	for _, want := range []string{
		"golaw gen --check: não medido sqlc: go is not on PATH", "golaw gen --check: deriva p.go difere da sua renderização: rode golaw gen",
		"golaw gen --check: deriva p.go arquivo regenerável ausente: rode golaw gen",
		"golaw gen --check: deriva p.go o contrato da entrada do manifesto sumiu: rode golaw gen",
		"golaw gen --check: deriva p.go difere do que o seu gerador fixado escreve: rode golaw gen",
		"golaw gen --check: deriva p.go não tem os stubs do seu contrato: rode golaw gen",
		"golaw gen --check: deriva p.go other: rode golaw gen", "golaw gen --check: 6 derivas, 0 lacunas abertas",
	} {
		if !slices.Contains(lines, want) {
			t.Errorf("lines lack %q: %q", want, lines)
		}
	}
	if got := checkPersonLines(nil, nil, nil); !reflect.DeepEqual(got, []string{"golaw gen --check: limpo, 0 derivas, 0 lacunas abertas"}) {
		t.Errorf("clean = %q", got)
	}
}

func TestGenWritesTheJSONReport(t *testing.T) {
	t.Parallel()
	ta, _ := newPort(t)
	if code := ta.run(t.Context(), []string{"gen", "--json"}); code != exitClean {
		t.Fatalf("exit = %d: %s", code, ta.stderr.String())
	}
	var doc genJSON
	if err := json.Unmarshal(ta.stdout.Bytes(), &doc); err != nil || doc.Written == nil || ta.stderr.Len() != 0 {
		t.Errorf("JSON = %+v, %v, stderr %q", doc, err, ta.stderr.String())
	}
}

func TestGenRefusesARepositoryItCannotRead(t *testing.T) {
	t.Parallel()
	ta, _ := genApp(t)
	ta.discover = func(_ context.Context, dir string) (string, error) {
		return "", fmt.Errorf("%w: %s", errNoRepository, dir)
	}
	for _, args := range [][]string{{"gen"}, {"gen", "--check"}} {
		if code := ta.run(t.Context(), args); code != exitFailure || !strings.Contains(ta.stderr.String(), "find the repository") {
			t.Errorf("%q exit = %d, stderr %q", args, code, ta.stderr.String())
		}
	}
}

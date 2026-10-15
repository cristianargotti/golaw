package goast

import (
	"errors"
	"go/token"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

var (
	errNever = errors.New("never")
	errBoom  = errors.New("boom")
)

// tree builds an in-memory snapshot from path to text.
func tree(files map[string]string) *proof.Tree {
	content := make(map[string][]byte, len(files))
	for p, text := range files {
		content[p] = []byte(text)
	}
	return proof.NewTree(content)
}

func TestTargetParsesOnceAndReportsFailures(t *testing.T) {
	t.Parallel()
	good := &target{file: model.File{Path: "a.go"}, src: []byte("package a\n")}
	first, fset, err := good.parsed()
	if err != nil || first == nil || fset == nil {
		t.Fatalf("parsed() = %v, %v, %v", first, fset, err)
	}
	if again, _, err := good.parsed(); err != nil || again != first {
		t.Error("parsed() must cache the tree")
	}
	bad := &target{file: model.File{Path: "b.go"}, src: []byte("package b\nfunc {")}
	if _, _, err := bad.parsed(); err == nil || !strings.HasPrefix(err.Error(), "parse b.go: ") {
		t.Errorf("parsed() on a broken file = %v", err)
	}
	if bad.dir() != "." || (&target{file: model.File{Path: "x/y/z.go"}}).dir() != "x/y" {
		t.Error("dir() is wrong")
	}
}

func TestFindingCarriesThePosition(t *testing.T) {
	t.Parallel()
	tg := &target{file: model.File{Path: "a.go"}, src: []byte("package a\n\nvar x = 1\n")}
	whole := tg.finding("code", token.NoPos, "whole file")
	if whole.Line != 0 || whole.Column != 0 || whole.Path != "a.go" || whole.Severity != model.SeverityDeny {
		t.Errorf("whole-file finding = %+v", whole)
	}
	file, _, err := tg.parsed()
	if err != nil {
		t.Fatal(err)
	}
	placed := tg.finding("code", file.Decls[0].Pos(), "at the var")
	if placed.Line != 3 || placed.Column != 1 || placed.Code != "code" || placed.Message != "at the var" {
		t.Errorf("placed finding = %+v", placed)
	}
}

func TestStateMemoAndSiblings(t *testing.T) {
	t.Parallel()
	st := newState(model.Input{Snapshot: tree(map[string]string{
		"a/x.go": "package a\n", "a/x_test.go": "package a\n", "a/b/y.go": "package b\n", "a/notes.md": "x",
	})})
	if !st.first("k") || st.first("k") || !st.first("other") {
		t.Error("first must answer true once per key")
	}
	if len(st.redacted) != 0 {
		t.Error("a new state has no redacting types")
	}
	siblings := st.siblings("a")
	if len(siblings) != 2 || siblings[0].Path != "a/x.go" || siblings[1].Class != model.ClassTest {
		t.Errorf("siblings(a) = %+v", siblings)
	}
	if len(st.siblings("a/b")) != 1 || len(st.siblings("c")) != 0 {
		t.Error("siblings must follow the directory exactly")
	}
}

func TestGatesAndPrefixes(t *testing.T) {
	t.Parallel()
	in := model.Input{Snapshot: tree(map[string]string{"sqlc.yaml": "x", "internal/adapters/a/b.go": "package b\n"})}
	if status, ok := requireFile("sqlc.yaml")(in); !ok || status.Kind != 0 {
		t.Errorf("requireFile present = %+v, %v", status, ok)
	}
	if status, ok := requireFile("go.work")(in); ok || status.Reason != "go.work is absent" {
		t.Errorf("requireFile absent = %+v, %v", status, ok)
	}
	if status, ok := requireDir("internal/adapters")(in); !ok || status.Kind != 0 {
		t.Errorf("requireDir present = %+v, %v", status, ok)
	}
	if status, ok := requireDir("internal/ports")(in); ok || status.Reason != "internal/ports is absent" {
		t.Errorf("requireDir absent = %+v, %v", status, ok)
	}
	if !under("cmd/app/main.go", "configs/", cmdDir) || under("internal/cmd/x.go", cmdDir) || under("a.go") {
		t.Error("under must match root-level prefixes only")
	}
	if err := parseError("a.go", errBoom); err.Error() != "parse a.go: boom" {
		t.Errorf("parseError = %v", err)
	}
}

func TestRunHonoursTheGateAndTheSpec(t *testing.T) {
	t.Parallel()
	closed := check{
		spec:  spec("gated"),
		gate:  func(model.Input) (model.Status, bool) { return model.NotApplicable("closed"), false },
		judge: func(*target, *state) ([]model.Finding, error) { return nil, errNever },
	}
	res, err := closed.Run(t.Context(), model.Input{Snapshot: tree(map[string]string{"a.go": "package a\n"})})
	if err != nil || res.Status.Kind != model.StatusNotApplicable || res.Status.Reason != "closed" {
		t.Errorf("gated Run = %+v, %v", res, err)
	}
	s := closed.Spec()
	if s.ID != "goast.gated" || s.Proof != "goast.gated" || s.Rule != rule || s.Needs[0] != capability {
		t.Errorf("spec = %+v", s)
	}
	if spec("x", "api").Needs[1] != "api" {
		t.Error("spec must append the extra needs")
	}
	failing := check{spec: spec("failing"), judge: func(*target, *state) ([]model.Finding, error) { return nil, errBoom }}
	snapshot := tree(map[string]string{"a.go": "package a\n"})
	res, err = failing.Run(t.Context(), model.Input{Snapshot: snapshot, Targets: snapshot.Files()})
	if err != nil || res.Status.Kind != model.StatusUnmeasured || res.Status.Reason != "boom" {
		t.Errorf("failing Run = %+v, %v", res, err)
	}
}

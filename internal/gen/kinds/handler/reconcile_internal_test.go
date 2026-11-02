package handler

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/holes"
	"github.com/cristianargotti/golaw/internal/gentest"
	"github.com/cristianargotti/golaw/internal/model"
)

// snapshotOf is the fixture tree with the owned files of v written once by
// golaw new, then edited by extra: a nil value deletes the path.
func snapshotOf(t *testing.T, v view, extra map[string][]byte) (model.Snapshot, []string) {
	t.Helper()
	files, err := owned(v)
	if err != nil {
		t.Fatal(err)
	}
	tree := gentest.Contracts()
	for _, f := range files {
		tree[f.Path] = f.Content
	}
	for path, content := range extra {
		if content == nil {
			delete(tree, path)
			continue
		}
		tree[path] = content
	}
	return gentest.Snapshot(t, tree), pathsOf(files)
}

func lineOf(t *testing.T, content []byte, prefix string) int {
	t.Helper()
	for i, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(line, prefix) {
			return i + 1
		}
	}
	t.Fatalf("no line starts with %q", prefix)
	return 0
}

func TestReconcileAppendsTheStubOfANewOperation(t *testing.T) {
	t.Parallel()
	before := viewOf(t, operations()[:1])
	s, paths := snapshotOf(t, before, nil)
	after := viewOf(t, operations())
	r, err := reconcile(after, s, paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Owned) != 0 || len(r.Decisions) != 0 || !slices.Equal(r.Paths, paths) || len(r.Appends) != 2 {
		t.Fatalf("reconcile = %+v", r)
	}
	code, test := r.Appends[0], r.Appends[1]
	if code.Path != handlerDir+"plans.go" || !slices.Equal(code.Holes, []string{"plansapi.archive-plan"}) || len(code.Decls) != 1 ||
		!strings.HasPrefix(string(code.Decls[0]), "// ArchivePlan serves POST /plans/{id}/archive.\n") {
		t.Errorf("code append = %+v\n%s", code, code.Decls)
	}
	if test.Path != handlerDir+"plans_test.go" || len(test.Holes) != 0 || len(test.Decls) != 1 ||
		!strings.Contains(flat(test.Decls[0]), "func TestArchivePlanHoleIsOpen(t *testing.T) {") {
		t.Errorf("test append = %+v\n%s", test, test.Decls)
	}
	content, err := s.Read(code.Path)
	if err != nil {
		t.Fatal(err)
	}
	appended, err := holes.Append(content, code)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(appended, content[:len(content)-1]) {
		t.Error("the append rewrote a byte of the owned file before the stub")
	}
	requireHoles(t, code.Path, appended, after.Ops)
}

func TestReconcileReportsARemovedOperationAsADecision(t *testing.T) {
	t.Parallel()
	s, paths := snapshotOf(t, viewOf(t, operations()), nil)
	content, err := s.Read(handlerDir + "plans.go")
	if err != nil {
		t.Fatal(err)
	}
	r, err := reconcile(viewOf(t, operations()[:1]), s, paths)
	if err != nil {
		t.Fatal(err)
	}
	want := []genkind.Decision{{
		Path: handlerDir + "plans.go", Line: lineOf(t, content, "func (h *Handler) ArchivePlan("),
		Text: "(*Handler).ArchivePlan is no longer in api/openapi.yaml: delete it or restore the operation in api/openapi.yaml",
	}}
	if !slices.Equal(r.Decisions, want) || len(r.Appends) != 0 || len(r.Owned) != 0 {
		t.Errorf("reconcile = %+v, want the decision %+v", r, want)
	}
}

func TestReconcileLeavesMethodsItCannotRecognize(t *testing.T) {
	t.Parallel()
	v := viewOf(t, operations())
	files, err := owned(v)
	if err != nil {
		t.Fatal(err)
	}
	plans := byPath(files)[handlerDir+"plans.go"]
	for name, added := range map[string]string{
		"near: an unexported helper method":              "\nfunc (h *Handler) problem(err error) apigen.Problem {\n\treturn apigen.Problem{Title: err.Error()}\n}\n",
		"near: an exported method of no operation shape": "\n// Routes lists the routes the handler serves.\nfunc (h *Handler) Routes() []string {\n\treturn nil\n}\n",
		"near: a function that is no method":             "\nfunc GetPlanOf(h *Handler) *Handler {\n\treturn h\n}\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s, paths := snapshotOf(t, v, map[string][]byte{handlerDir + "plans.go": append(slices.Clone(plans), added...)})
			r, err := reconcile(v, s, paths)
			if err != nil || len(r.Decisions) != 0 || len(r.Appends) != 0 || len(r.Owned) != 0 {
				t.Errorf("reconcile = %+v, %v, want nothing to do", r, err)
			}
		})
	}
}

func TestReconcileCreatesTheFilesANewChunkNeeds(t *testing.T) {
	t.Parallel()
	moved := operations()
	moved[1].Tag = "admin"
	tests := map[string]struct {
		before, after []genkind.Operation
		want          []string
	}{
		"red: an operation under a new tag": {
			before: operations()[:1], after: moved, want: []string{handlerDir + "admin.go", handlerDir + "admin_test.go"},
		},
		"red: an operation past a full chunk": {
			before: manyOperations(opsPerFile, "stock"), after: manyOperations(opsPerFile+1, "stock"),
			want: []string{handlerDir + "stock_2.go", handlerDir + "stock_2_test.go"},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s, paths := snapshotOf(t, viewOf(t, tc.before), nil)
			after := viewOf(t, tc.after)
			r, err := reconcile(after, s, paths)
			if err != nil {
				t.Fatal(err)
			}
			if got := pathsOf(r.Owned); !slices.Equal(got, tc.want) || len(r.Appends) != 0 || len(r.Decisions) != 0 {
				t.Fatalf("owned = %v, appends %+v, decisions %+v", got, r.Appends, r.Decisions)
			}
			if !slices.IsSorted(r.Paths) || !slices.Contains(r.Paths, tc.want[0]) || !slices.Contains(r.Paths, tc.want[1]) {
				t.Errorf("entry paths = %v", r.Paths)
			}
			requireHoles(t, tc.want[0], byPath(r.Owned)[tc.want[0]], after.Ops[len(after.Ops)-1:])
		})
	}
}

func TestReconcileNeverWritesOutsideItsEntry(t *testing.T) {
	t.Parallel()
	tree := gentest.Contracts()
	tree[handlerDir+"plans.go"] = []byte("package plansapi\n")
	r, err := reconcile(viewOf(t, operations()[:1]), gentest.Snapshot(t, tree), []string{handlerDir + "status.go"})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Owned) != 0 || len(r.Appends) != 0 || len(r.Decisions) != 1 || r.Decisions[0].Path != handlerDir+"plans.go" ||
		!strings.Contains(r.Decisions[0].Text, "exists outside the manifest entry of handler plansapi") {
		t.Errorf("reconcile = %+v, want one decision naming plans.go", r)
	}
}

func TestReconcileAppendsOnlyTheRedTestsThatAreMissing(t *testing.T) {
	t.Parallel()
	files, err := owned(viewOf(t, operations()))
	if err != nil {
		t.Fatal(err)
	}
	tests := byPath(files)[handlerDir+"plans_test.go"]
	for name, tc := range map[string]struct {
		testFile []byte
		appends  int
		owned    []string
	}{
		"near: the red test is already written":                               {testFile: tests, appends: 1},
		"red: a deleted red test file is written again for the new operation": {owned: []string{handlerDir + "plans_test.go"}, appends: 1},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s, paths := snapshotOf(t, viewOf(t, operations()[:1]), map[string][]byte{handlerDir + "plans_test.go": tc.testFile})
			r, err := reconcile(viewOf(t, operations()), s, paths)
			if err != nil {
				t.Fatal(err)
			}
			if len(r.Appends) != tc.appends || r.Appends[0].Path != handlerDir+"plans.go" || !slices.Equal(pathsOf(r.Owned), tc.owned) {
				t.Errorf("appends %+v, owned %v", r.Appends, pathsOf(r.Owned))
			}
		})
	}
}

func TestReconcileCannotJudgeAnOwnedFileThatDoesNotParse(t *testing.T) {
	t.Parallel()
	v := viewOf(t, operations())
	s, paths := snapshotOf(t, v, map[string][]byte{handlerDir + "plans.go": []byte("package plansapi\n\nfunc (\n")})
	if _, err := reconcile(v, s, paths); !errors.Is(err, genkind.ErrCannotJudge) || !strings.Contains(err.Error(), "plans.go") {
		t.Fatalf("reconcile = %v, want ErrCannotJudge naming the file", err)
	}
}

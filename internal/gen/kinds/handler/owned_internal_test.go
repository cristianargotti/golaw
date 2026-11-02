package handler

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/holes"
	"github.com/cristianargotti/golaw/internal/genheader"
)

func requireHoles(t *testing.T, path string, content []byte, want []operation) {
	t.Helper()
	found, err := holes.Find(path, content)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != len(want) {
		t.Fatalf("%s opens %d holes, want %d: %+v", path, len(found), len(want), found)
	}
	for i, h := range found {
		op := want[i]
		if h.ID != op.Hole || h.Constraint != op.Constraint || h.Func != "(*Handler)."+op.Name || h.Path != path {
			t.Errorf("hole %d of %s = %+v, want %s", i, path, h, op.Hole)
		}
	}
}

func TestOwnedFilesOpenOneHolePerOperation(t *testing.T) {
	t.Parallel()
	v := viewOf(t, operations())
	files, err := owned(v)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{handlerDir + "plans.go", handlerDir + "plans_test.go", handlerDir + "status.go", handlerDir + "status_internal_test.go"}
	if got := pathsOf(files); !slices.Equal(got, want) {
		t.Fatalf("paths = %v, want %v", got, want)
	}
	all := byPath(files)
	for path, content := range all {
		if _, ok := genheader.Parse(content); ok {
			t.Errorf("%s carries the golaw header: an owned file belongs to the repository", path)
		}
	}
	requireGo(t, all)
	requireLaws(t, all)
	requireHoles(t, handlerDir+"plans.go", all[handlerDir+"plans.go"], v.Ops)
	id, constraint, _ := statusHole(v)
	status, err := holes.Find(handlerDir+"status.go", all[handlerDir+"status.go"])
	if err != nil || len(status) != 1 || status[0].ID != id || status[0].Constraint != constraint || status[0].Func != "statusRules" {
		t.Errorf("status holes = %+v, %v", status, err)
	}
	for path, wants := range map[string][]string{
		handlerDir + "plans.go": {
			"func (h *Handler) GetPlan(ctx context.Context, request apigen.GetPlanRequestObject) (apigen.GetPlanResponseObject, error) {",
			`return nil, gensupport.Hole("plansapi.get-plan", ctx, request)`,
			`apigen "example.com/slice/api/gen"`,
		},
		handlerDir + "plans_test.go": {
			"package plansapi_test",
			"func TestGetPlanHoleIsOpen(t *testing.T) { t.Parallel()",
			"_, err := newTestHandler().GetPlan(t.Context(), filled[apigen.GetPlanRequestObject]())",
			"if errors.Is(err, gensupport.ErrNotImplemented) {",
			`t.Fatal("hole plansapi.get-plan is open: map the request`,
		},
		handlerDir + "status.go": {
			"// Package plansapi serves api/openapi.yaml over the use cases of growth. package plansapi",
			"func statusRules() []statusRule {", "return []statusRule{}",
		},
		handlerDir + "status_internal_test.go": {"package plansapi", "if len(statusRules()) == 0 {", `t.Fatal("hole plansapi.status-rules is open: `},
	} {
		for _, w := range wants {
			if !strings.Contains(flat(all[path]), w) {
				t.Errorf("%s lacks %q:\n%s", path, w, all[path])
			}
		}
	}
}

func TestOwnedFilesSplitAtTheOperationLimit(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		ops   int
		split bool
	}{
		"red: one operation over the limit opens a second file": {ops: opsPerFile + 1, split: true},
		"near: the limit itself stays in one file":              {ops: opsPerFile},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			v := viewOf(t, manyOperations(tc.ops, "items"))
			files, err := owned(v)
			if err != nil {
				t.Fatal(err)
			}
			all := byPath(files)
			requireGo(t, all)
			requireHoles(t, handlerDir+"items.go", all[handlerDir+"items.go"], v.Ops[:min(tc.ops, opsPerFile)])
			_, code := all[handlerDir+"items_2.go"]
			_, test := all[handlerDir+"items_2_test.go"]
			if code != tc.split || test != tc.split {
				t.Fatalf("second chunk present %v and %v, want %v: %v", code, test, tc.split, pathsOf(files))
			}
			if tc.split {
				requireHoles(t, handlerDir+"items_2.go", all[handlerDir+"items_2.go"], v.Ops[opsPerFile:])
			}
		})
	}
}

func TestOwnedFilesFollowTheOperationTags(t *testing.T) {
	t.Parallel()
	ops := operations()
	ops[1].Tag = "Plan Admin"
	v := viewOf(t, ops)
	files, err := owned(v)
	if err != nil {
		t.Fatal(err)
	}
	all := byPath(files)
	requireHoles(t, handlerDir+"plans.go", all[handlerDir+"plans.go"], v.Ops[:1])
	requireHoles(t, handlerDir+"plan_admin.go", all[handlerDir+"plan_admin.go"], v.Ops[1:])
	if _, ok := all[handlerDir+"plan_admin_test.go"]; !ok {
		t.Errorf("the second tag has no red test file: %v", pathsOf(files))
	}
}

func TestChunkPathsNameEveryChunk(t *testing.T) {
	t.Parallel()
	for n, want := range map[int][2]string{
		1: {"internal/adapters/in/plansapi/plans.go", "internal/adapters/in/plansapi/plans_test.go"},
		3: {"internal/adapters/in/plansapi/plans_3.go", "internal/adapters/in/plansapi/plans_3_test.go"},
	} {
		if code, test := chunkPaths("internal/adapters/in/plansapi", "plans", n); code != want[0] || test != want[1] {
			t.Errorf("chunkPaths(%d) = %s, %s", n, code, test)
		}
	}
	if got := codeImports(viewOf(t, operations())); !slices.ContainsFunc(got, func(i genkind.Import) bool { return i.Path == "context" }) {
		t.Errorf("codeImports = %+v", got)
	}
}

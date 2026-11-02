package handler

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gentest"
)

func TestNewViewRefusesWhatItCannotServe(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		edit func(t *testing.T, in *genkind.Input, req *genkind.Request)
		want error
	}{
		"red: a name that is not a package": {
			edit: func(_ *testing.T, _ *genkind.Input, r *genkind.Request) { r.Name.Area = "Plans-API" }, want: genkind.ErrRequest,
		},
		"red: a Go keyword": {edit: func(_ *testing.T, _ *genkind.Input, r *genkind.Request) { r.Name.Area = "func" }, want: genkind.ErrRequest},
		"red: an import the handler files need": {
			edit: func(_ *testing.T, _ *genkind.Input, r *genkind.Request) { r.Name.Area = "apigen" }, want: genkind.ErrRequest,
		},
		"red: a type other than Handler": {
			edit: func(_ *testing.T, _ *genkind.Input, r *genkind.Request) { r.Name.Type = "Server" }, want: genkind.ErrRequest,
		},
		"red: a document the generate block does not name": {
			edit: func(_ *testing.T, _ *genkind.Input, r *genkind.Request) { r.API = "api/other.yaml" }, want: genkind.ErrRequest,
		},
		"red: no use case area": {edit: func(_ *testing.T, _ *genkind.Input, r *genkind.Request) { r.Usecases = nil }, want: genkind.ErrRequest},
		"red: an area without a use case": {
			edit: func(_ *testing.T, _ *genkind.Input, r *genkind.Request) { r.Usecases = []string{"billing"} }, want: genkind.ErrRequest,
		},
		"red: an area named twice": {
			edit: func(_ *testing.T, _ *genkind.Input, r *genkind.Request) { r.Usecases = []string{"growth", "growth"} }, want: genkind.ErrRequest,
		},
		"red: an area named like the handler": {
			edit: func(_ *testing.T, _ *genkind.Input, r *genkind.Request) { r.Name.Area = "growth" }, want: genkind.ErrRequest,
		},
		"red: eight areas pass the parameter limit": {
			edit: func(_ *testing.T, _ *genkind.Input, r *genkind.Request) {
				r.Usecases = []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel"}
			},
			want: genkind.ErrRequest,
		},
		"red: a port without its generated fake": {
			edit: func(_ *testing.T, in *genkind.Input, _ *genkind.Request) {
				in.Manifest = without(in.Manifest, genkind.KindPort, "growth.Clock")
			},
			want: genkind.ErrRequest,
		},
		"red: the server file is not written yet": {
			edit: func(t *testing.T, in *genkind.Input, _ *genkind.Request) {
				t.Helper()
				in.Snapshot = gentest.Snapshot(t, filesWithout("api/gen/server.gen.go"))
			},
			want: genkind.ErrRequest,
		},
		"red: the server declares no operation": {
			edit: func(_ *testing.T, in *genkind.Input, _ *genkind.Request) { in.Contracts = gentest.FakeContracts{} }, want: genkind.ErrRequest,
		},
		"red: a handler of that name exists": {
			edit: func(_ *testing.T, in *genkind.Input, _ *genkind.Request) {
				in.Manifest = in.Manifest.With(handlerEntry("plansapi"))
			},
			want: genkind.ErrExists,
		},
		"red: another handler serves the same server": {
			edit: func(_ *testing.T, in *genkind.Input, _ *genkind.Request) {
				in.Manifest = in.Manifest.With(handlerEntry("adminapi"))
			},
			want: genkind.ErrExists,
		},
		"red: the regenerable file exists without an entry": {
			edit: func(t *testing.T, in *genkind.Input, _ *genkind.Request) {
				t.Helper()
				files := gentest.Contracts()
				files[handlerDir+"zz_generated_handler.go"] = []byte("package plansapi\n")
				in.Snapshot = gentest.Snapshot(t, files)
			},
			want: genkind.ErrExists,
		},
		"near: the document defaults to the generate block": {
			edit: func(_ *testing.T, _ *genkind.Input, r *genkind.Request) { r.API = "" },
		},
		"near: the type spelled out":  {edit: func(_ *testing.T, _ *genkind.Input, r *genkind.Request) { r.Name.Type = "Handler" }},
		"near: the canonical request": {edit: func(*testing.T, *genkind.Input, *genkind.Request) {}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			in, req := input(t), request()
			tc.edit(t, &in, &req)
			if _, err := newView(in, req); !errors.Is(err, tc.want) {
				t.Fatalf("newView = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestNewViewDerivesTheShapeFromTheContracts(t *testing.T) {
	t.Parallel()
	v := viewOf(t, operations())
	if v.Package != "plansapi" || v.Dir != "internal/adapters/in/plansapi" || v.API != "api/openapi.yaml" || v.Server != "api/gen/server.gen.go" {
		t.Errorf("view paths = %+v", v)
	}
	if v.APIGen != (genkind.Import{Name: "apigen", Path: module + "/api/gen"}) || v.Support.Path != module+"/internal/gensupport" ||
		v.Self.Path != module+"/internal/adapters/in/plansapi" {
		t.Errorf("view imports = %+v, %+v, %+v", v.APIGen, v.Support, v.Self)
	}
	wantFakes := []fake{
		{Port: "Clock", Ident: "clocktest", Import: genkind.Import{Path: module + "/internal/application/growth/clocktest"}},
		{Port: "Ledger", Ident: "ledgertest", Import: genkind.Import{Path: module + "/internal/application/growth/ledgertest"}},
	}
	if len(v.Areas) != 1 || v.Areas[0].Name != "growth" || v.Areas[0].Field != "uc" ||
		v.Areas[0].Import.Path != module+"/internal/application/growth" || !reflect.DeepEqual(v.Areas[0].Fakes, wantFakes) {
		t.Errorf("areas = %+v", v.Areas)
	}
	want := []struct{ name, pattern, hole, action string }{
		{"GetPlan", "GET /plans/{id}", "plansapi.get-plan", `gensupport.Prefix("plansapi: get plan")`},
		{"ArchivePlan", "POST /plans/{id}/archive", "plansapi.archive-plan", `gensupport.Prefix("plansapi: archive plan")`},
	}
	if len(v.Ops) != len(want) {
		t.Fatalf("ops = %+v", v.Ops)
	}
	for i, w := range want {
		op := v.Ops[i]
		if op.Name != w.name || op.Pattern != w.pattern || op.Stem != "plans" || op.Hole != w.hole ||
			!strings.Contains(op.Constraint, w.action) || !strings.Contains(op.Constraint, "h.uc") ||
			op.Message != "hole "+w.hole+" is open: "+op.Constraint {
			t.Errorf("op %d = %+v", i, op)
		}
		if strings.ContainsAny(op.Constraint, "%\n") {
			t.Errorf("constraint %q carries a format verb or a newline", op.Constraint)
		}
	}
	id, constraint, message := statusHole(v)
	if id != "plansapi.status-rules" || !strings.Contains(constraint, "growth") || message != "hole "+id+" is open: "+constraint {
		t.Errorf("statusHole = %q, %q, %q", id, constraint, message)
	}
}

func TestBuildAliasesFakesThatShareAName(t *testing.T) {
	t.Parallel()
	in := input(t)
	in.Manifest = withBilling(in.Manifest)
	v, err := build(in, "plansapi", "api/openapi.yaml", "api/gen/server.gen.go", []string{"growth", "billing"})
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Areas) != 2 || v.Areas[0].Name != "billing" || v.Areas[1].Name != "growth" {
		t.Fatalf("areas = %+v, want billing then growth", v.Areas)
	}
	if got := v.Areas[0].Fakes[0]; got.Ident != "ledgertest" || got.Import.Name != "" {
		t.Errorf("billing fake = %+v, want the plain package name", got)
	}
	if got := v.Areas[1].Fakes[1]; got.Ident != "growthledgertest" || got.Import.Name != "growthledgertest" {
		t.Errorf("growth fake = %+v, want an alias after the area", got)
	}
	if !strings.Contains(v.Ops[0].Constraint, "h.ucBilling or h.ucGrowth") {
		t.Errorf("constraint = %q", v.Ops[0].Constraint)
	}
}

func TestFieldNamesNameEachArea(t *testing.T) {
	t.Parallel()
	for areas, want := range map[string][]string{
		"growth":         {"uc"},
		"billing growth": {"ucBilling", "ucGrowth"},
	} {
		if got := fieldNames(strings.Fields(areas)); !reflect.DeepEqual(got, want) {
			t.Errorf("fieldNames(%s) = %v, want %v", areas, got, want)
		}
	}
}

func TestStemKeepsOneOwnedFilePerTag(t *testing.T) {
	t.Parallel()
	for tag, want := range map[string]string{
		"plans":        "plans",
		"Plan Admin":   "plan_admin",
		"billing-v2":   "billing_v2",
		"":             "default",
		"status":       "status_api",
		"handler":      "handler_api",
		"plans_test":   "plans_test_api",
		"plans_2":      "plans_2_api",
		"2fa":          "api_2fa",
		"zz_generated": "api_zz_generated",
	} {
		if got := stem(tag); got != want {
			t.Errorf("stem(%q) = %q, want %q", tag, got, want)
		}
	}
}

package genkind_test

import (
	"context"
	"errors"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/model"
)

// stub is the smallest kind and wirer: it answers its request as an entry,
// which proves the interfaces carry what a kind needs and nothing more.
type stub struct{}

var (
	_ genkind.Kind  = stub{}
	_ genkind.Wirer = stub{}
)

func (stub) Name() string  { return genkind.KindPort }
func (stub) Template() int { return 1 }

func (stub) New(_ context.Context, in genkind.Input, req genkind.Request) (genkind.Output, error) {
	if req.Name.Type == "" {
		return genkind.Output{}, genkind.ErrRequest
	}
	return genkind.Output{
		Entry:       manifest.Entry{Kind: genkind.KindPort, Template: "port@1", Name: req.Name.String()},
		Regenerable: []genkind.File{{Path: in.Policy.Layout.Port + "/x.go", Content: []byte("package x\n")}},
		Needs:       []genkind.Need{{Kind: genkind.KindUsecase, Request: genkind.Request{Name: genkind.Name{Area: req.Name.Area, Type: req.Usecase}}}},
	}, nil
}

func (stub) Regenerate(_ context.Context, _ genkind.Input, entry manifest.Entry) (genkind.Output, error) {
	return genkind.Output{Entry: entry}, nil
}

func (stub) Wire(_ context.Context, _ genkind.Input, main string, bindings []genkind.Binding) (genkind.Output, error) {
	return genkind.Output{Regenerable: []genkind.File{{Path: "cmd/" + main + "/zz_generated_wiring.go"}}, Bindings: bindings}, nil
}

func TestTheKindNamesAreTheManifestWords(t *testing.T) {
	t.Parallel()
	words := map[string]string{
		genkind.KindPort: "port", genkind.KindAdapterSQL: "adapter-sql", genkind.KindAdapterHTTP: "adapter-http",
		genkind.KindUsecase: "usecase", genkind.KindHandler: "handler", genkind.KindWiring: "wiring", genkind.KindSupport: "support",
	}
	for got, want := range words {
		if got != want {
			t.Errorf("kind constant %q, want %q", got, want)
		}
	}
	if got := (genkind.Name{Area: "growth", Type: "Ledger"}).String(); got != "growth.Ledger" {
		t.Errorf("Name.String() = %q", got)
	}
}

func TestAKindRendersFromItsInputAlone(t *testing.T) {
	t.Parallel()
	in := genkind.Input{Module: "example.com/slice", Version: "0.3.0", Policy: model.DefaultGenerate()}
	out, err := stub{}.New(t.Context(), in, genkind.Request{Name: genkind.Name{Area: "growth", Type: "Ledger"}, Usecase: "ReadPlan"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Entry.Name != "growth.Ledger" || out.Regenerable[0].Path != "internal/application/{area}/x.go" {
		t.Errorf("output = %+v", out)
	}
	if len(out.Needs) != 1 || out.Needs[0].Kind != genkind.KindUsecase || out.Needs[0].Request.Name.Type != "ReadPlan" {
		t.Errorf("needs = %+v", out.Needs)
	}
	if _, reqErr := (stub{}).New(t.Context(), in, genkind.Request{}); !errors.Is(reqErr, genkind.ErrRequest) {
		t.Errorf("an empty request = %v, want ErrRequest", reqErr)
	}
	wired, err := stub{}.Wire(t.Context(), in, "api", []genkind.Binding{{Kind: genkind.KindAdapterSQL, Port: "Ledger", Call: "postgres.NewPlans(infra.DB)"}})
	if err != nil || wired.Regenerable[0].Path != "cmd/api/zz_generated_wiring.go" || len(wired.Bindings) != 1 {
		t.Errorf("Wire = %+v, %v", wired, err)
	}
}

func TestTheSentinelsAreDistinct(t *testing.T) {
	t.Parallel()
	all := []error{genkind.ErrCannotJudge, genkind.ErrExists, genkind.ErrRequest}
	for i, a := range all {
		for j, b := range all {
			if i != j && errors.Is(a, b) {
				t.Errorf("%v matches %v", a, b)
			}
		}
	}
}

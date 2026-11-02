package adapterhttp_test

import (
	"bytes"
	"go/parser"
	"go/token"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/contract"
	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/holes"
	"github.com/cristianargotti/golaw/internal/gen/kinds/adapterhttp"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/genheader"
	"github.com/cristianargotti/golaw/internal/gentest"
	"github.com/cristianargotti/golaw/internal/model"
)

const (
	sliceModule = "example.com/slice"
	portsPath   = "internal/application/growth/ports.go"
	dir         = "internal/adapters/out/planapi"
)

const (
	readConstraint    = "build the URL from c.base and the parameters; read it with gensupport.Get(ctx, c.client, url); map http.StatusNotFound to growth.ErrPlanNotFound; gate the status with gensupport.Status and decode the body with gensupport.Decode; wrap every error with gensupport.Prefix(actionClientReadPlan)"
	archiveConstraint = "build the URL from c.base and the parameters; send it with gensupport.Send(ctx, c.client, method, url, body); map http.StatusNotFound to growth.ErrPlanNotFound; gate the status with gensupport.Status; wrap every error with gensupport.Prefix(actionClientArchivePlan)"
	seedConstraint    = "serve a tlstest.Routes with tlstest.Serve(t, routes) and answer ledgertest.Subject{Port: planapi.NewClient(planapi.ClientConfig{BaseURL: server.URL, Client: server.Client()}), Seed: add the routes each case's Args and Want need}"
)

func portEntry() manifest.Entry {
	return manifest.Entry{
		Kind: genkind.KindPort, Template: "port@1", Name: "growth.Ledger",
		Contracts: []manifest.Contract{{Path: portsPath, Symbol: "Ledger"}},
	}
}

func input(t *testing.T, files map[string][]byte, entries ...manifest.Entry) genkind.Input {
	t.Helper()
	m := manifest.Manifest{Schema: manifest.Schema, Support: manifest.Support{Path: "internal/gensupport", Template: "support@1"}}
	for i := range entries {
		m = m.With(entries[i])
	}
	snap := gentest.Snapshot(t, files)
	return genkind.Input{
		Module: sliceModule, Version: "0.3.0", Policy: model.DefaultGenerate(),
		Snapshot: snap, Manifest: m, Contracts: contract.Reader{Snapshot: snap},
	}
}

func request(typ string) genkind.Request {
	return genkind.Request{Name: genkind.Name{Area: "planapi", Type: typ}, Port: genkind.Name{Area: "growth", Type: "Ledger"}}
}

func newAdapter(t *testing.T, files map[string][]byte, typ string, entries ...manifest.Entry) genkind.Output {
	t.Helper()
	out, err := adapterhttp.Kind().New(t.Context(), input(t, files, append([]manifest.Entry{portEntry()}, entries...)...), request(typ))
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func byPath(files []genkind.File) map[string][]byte {
	out := make(map[string][]byte, len(files))
	for _, f := range files {
		out[f.Path] = f.Content
	}
	return out
}

func written(files map[string][]byte, out genkind.Output) map[string][]byte {
	next := maps.Clone(files)
	maps.Copy(next, byPath(out.Regenerable))
	maps.Copy(next, byPath(out.Owned))
	return next
}

func openHoles(t *testing.T, files map[string][]byte) map[string]string {
	t.Helper()
	found := make(map[string]string)
	for p, content := range files {
		list, err := holes.Find(p, content)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		for _, h := range list {
			found[h.ID] = h.Constraint
		}
	}
	return found
}

func TestKindNamesItself(t *testing.T) {
	t.Parallel()
	if k := adapterhttp.Kind(); k.Name() != genkind.KindAdapterHTTP || k.Template() != 1 {
		t.Errorf("Kind = %s@%d", k.Name(), k.Template())
	}
}

func TestNewWritesTheHTTPAdapterShape(t *testing.T) {
	t.Parallel()
	out := newAdapter(t, gentest.Contracts(), "Client")
	regenerable, owned := byPath(out.Regenerable), byPath(out.Owned)
	wantRegenerable := []string{dir + "/zz_generated_client.go", dir + "/zz_generated_client_contract_test.go"}
	wantOwned := []string{dir + "/client.go", dir + "/client_test.go", dir + "/doc.go"}
	if got := slices.Sorted(maps.Keys(regenerable)); !slices.Equal(got, wantRegenerable) {
		t.Errorf("regenerable = %v", got)
	}
	if got := slices.Sorted(maps.Keys(owned)); !slices.Equal(got, wantOwned) {
		t.Errorf("owned = %v", got)
	}
	entry := out.Entry
	if entry.Kind != genkind.KindAdapterHTTP || entry.Template != "adapter-http@1" || entry.Name != "planapi.Client" ||
		!slices.Equal(entry.Contracts, portEntry().Contracts) || !slices.Equal(entry.Regenerable, wantRegenerable) || !slices.Equal(entry.Owned, wantOwned) {
		t.Errorf("entry = %+v", entry)
	}
	for p, content := range regenerable {
		if h, ok := genheader.Parse(content); !ok || h.Kind != genkind.KindAdapterHTTP || h.Template != 1 {
			t.Errorf("%s header = %+v, %v", p, h, ok)
		}
	}
	wantBindings := []genkind.Binding{{
		Kind: genkind.KindAdapterHTTP, Area: "growth", Port: "Ledger", Import: genkind.Import{Path: sliceModule + "/" + dir},
		Call:  "planapi.NewClient(infra.PlanapiClient)",
		Needs: []genkind.InfraUse{{Field: "PlanapiClient", Type: "planapi.ClientConfig", Import: genkind.Import{Path: sliceModule + "/" + dir}}},
	}}
	if !reflect.DeepEqual(out.Bindings, wantBindings) {
		t.Errorf("bindings = %+v", out.Bindings)
	}
}

func TestNewRendersTheBoundedClient(t *testing.T) {
	t.Parallel()
	regenerable := byPath(newAdapter(t, gentest.Contracts(), "Client").Regenerable)
	client := string(regenerable[dir+"/zz_generated_client.go"])
	for _, want := range []string{
		"type ClientConfig = gensupport.HTTPConfig",
		"type Client struct {\n\tbase   string\n\tclient *http.Client\n}",
		"func NewClient(cfg ClientConfig) *Client {\n\tbase, client := cfg.Endpoint()\n\treturn &Client{base: base, client: client}\n}",
		"var _ growth.Ledger = (*Client)(nil)",
		"actionClientReadPlan    = \"planapi: client read plan\"",
	} {
		if !strings.Contains(client, want) {
			t.Errorf("zz_generated_client.go lacks %q", want)
		}
	}
	suite := string(regenerable[dir+"/zz_generated_client_contract_test.go"])
	for _, want := range []string{"ledgertest.Run(t, clientSubject)", "func TestNewClientBoundsItsDefaultClient(t *testing.T)", "planapi.NewClient(planapi.ClientConfig{})"} {
		if !strings.Contains(suite, want) {
			t.Errorf("zz_generated_client_contract_test.go lacks %q", want)
		}
	}
	if strings.Contains(suite, "//go:build") {
		t.Error("the HTTP contract test is tagged: it runs in the unit suite over the TLS test server")
	}
}

func TestNewOpensOneHoleWithItsRedTestPerMethod(t *testing.T) {
	t.Parallel()
	owned := byPath(newAdapter(t, gentest.Contracts(), "Client").Owned)
	want := map[string]string{"client.read-plan": readConstraint, "client.archive-plan": archiveConstraint, "seed.client": seedConstraint}
	if got := openHoles(t, owned); !maps.Equal(got, want) {
		t.Errorf("holes = %v, want %v", got, want)
	}
	stub := "func (c *Client) ReadPlan(ctx context.Context, account growth.AccountID) (growth.Plan, error) {\n" +
		"\t// golaw:hole client.read-plan " + readConstraint + "\n\tvar zero growth.Plan\n" +
		"\treturn zero, gensupport.Hole(\"client.read-plan\", actionClientReadPlan, c.base, c.client, ctx, account)\n}\n"
	red := "func TestClientReadPlan(t *testing.T) {\n\tt.Parallel()\n" +
		"\tserver := tlstest.Server(t, tlstest.Route{Status: http.StatusNotFound})\n" +
		"\tadapter := planapi.NewClient(planapi.ClientConfig{BaseURL: server.URL, Client: server.Client()})\n" +
		"\tvar account growth.AccountID\n\t_, err := adapter.ReadPlan(t.Context(), account)\n" +
		"\tif errors.Is(err, gensupport.ErrNotImplemented) {\n\t\tt.Fatal(\"hole client.read-plan is open: " + readConstraint + "\")\n\t}\n}\n"
	tests := []struct{ path, want string }{
		{dir + "/client.go", stub},
		{dir + "/client_test.go", red},
		{dir + "/client_test.go", "func clientSubject(t *testing.T) ledgertest.Subject {"},
		{dir + "/client_test.go", "\tif err := gensupport.Hole(\"seed.client\", t); err != nil {\n\t\tt.Fatalf(\"hole seed.client is open: " + seedConstraint + ": %v\", err)\n\t}\n"},
		{dir + "/doc.go", "// Package planapi holds the outbound adapters of its directory.\npackage planapi\n"},
	}
	for _, tc := range tests {
		if !bytes.Contains(owned[tc.path], []byte(tc.want)) {
			t.Errorf("%s lacks:\n%s\ngot:\n%s", tc.path, tc.want, owned[tc.path])
		}
	}
}

func TestNewObeysTheTemplateLaws(t *testing.T) {
	t.Parallel()
	first := newAdapter(t, gentest.Contracts(), "Client")
	if again := newAdapter(t, gentest.Contracts(), "Client"); !reflect.DeepEqual(first, again) {
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
	}
}

func TestASecondInstanceSharesThePackage(t *testing.T) {
	t.Parallel()
	client := newAdapter(t, gentest.Contracts(), "Client")
	mirror := newAdapter(t, written(gentest.Contracts(), client), "Mirror", client.Entry)
	owned := byPath(mirror.Owned)
	if _, ok := owned[dir+"/doc.go"]; ok {
		t.Error("the second instance rewrote doc.go")
	}
	if !bytes.Contains(byPath(mirror.Regenerable)[dir+"/zz_generated_mirror.go"], []byte("type MirrorConfig = gensupport.HTTPConfig")) {
		t.Error("the second instance does not declare its own configuration type")
	}
	if len(mirror.Bindings) != 0 {
		t.Errorf("the second adapter of the port bound itself: %+v", mirror.Bindings)
	}
}

func TestRegenerateAppendsTheNewMethodAndNamesTheOrphan(t *testing.T) {
	t.Parallel()
	out := newAdapter(t, gentest.Contracts(), "Client")
	files := written(gentest.Contracts(), out)
	before := files[dir+"/client.go"]
	files[dir+"/client.go"] = append(bytes.Clone(before), []byte("\n// PurgeAll purges every plan.\nfunc (c *Client) PurgeAll() error {\n\treturn nil\n}\n")...)
	files[portsPath] = bytes.Replace(files[portsPath], []byte("\tArchivePlan(ctx context.Context, account AccountID) error\n"),
		[]byte("\tArchivePlan(ctx context.Context, account AccountID) error\n\t// PurgePlans purges the plans of one account.\n\tPurgePlans(ctx context.Context, account AccountID) error\n"), 1)
	regen, err := adapterhttp.Kind().Regenerate(t.Context(), input(t, files, portEntry(), out.Entry), out.Entry)
	if err != nil {
		t.Fatal(err)
	}
	appends := make(map[string]genkind.Append)
	for _, a := range regen.Appends {
		appends[a.Path] = a
	}
	if a := appends[dir+"/client.go"]; len(a.Decls) != 1 || !slices.Equal(a.Holes, []string{"client.purge-plans"}) {
		t.Errorf("client.go append = %+v", a)
	}
	if a := appends[dir+"/client_test.go"]; len(a.Decls) != 1 || !bytes.Contains(a.Decls[0], []byte("func TestClientPurgePlans(t *testing.T)")) {
		t.Errorf("client_test.go append = %+v", a)
	}
	want := []genkind.Decision{{
		Path: dir + "/client.go", Line: bytes.Count(before, []byte("\n")) + 3,
		Text: "(*Client).PurgeAll is no longer in port Ledger: delete it or restore the method in internal/application/growth/ports.go",
	}}
	if !slices.Equal(regen.Decisions, want) {
		t.Errorf("decisions = %+v, want %+v", regen.Decisions, want)
	}
}

func TestRegenerateOfAnUnchangedContractChangesNothing(t *testing.T) {
	t.Parallel()
	out := newAdapter(t, gentest.Contracts(), "Client")
	regen, err := adapterhttp.Kind().Regenerate(t.Context(), input(t, written(gentest.Contracts(), out), portEntry(), out.Entry), out.Entry)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(regen.Regenerable, out.Regenerable) || len(regen.Appends) != 0 || len(regen.Decisions) != 0 || !reflect.DeepEqual(regen.Entry, out.Entry) {
		t.Errorf("regenerate of an unchanged contract = %+v", regen)
	}
}

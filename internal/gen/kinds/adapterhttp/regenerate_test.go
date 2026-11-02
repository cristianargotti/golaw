package adapterhttp_test

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/kinds/adapterhttp"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/gentest"
)

// withPurge adds a method to the Ledger port, so a regeneration has one to append.
func withPurge(files map[string][]byte) map[string][]byte {
	files[portsPath] = bytes.Replace(files[portsPath], []byte("\tArchivePlan(ctx context.Context, account AccountID) error\n"),
		[]byte("\tArchivePlan(ctx context.Context, account AccountID) error\n\t// PurgePlans purges the plans of one account.\n\tPurgePlans(ctx context.Context, account AccountID) error\n"), 1)
	return files
}

func TestRegenerateNamesTheGoneOwnedFiles(t *testing.T) {
	t.Parallel()
	out := newAdapter(t, gentest.Contracts(), "Client")
	files := withPurge(written(gentest.Contracts(), out))
	delete(files, dir+"/client.go")
	delete(files, dir+"/client_test.go")
	regen, err := adapterhttp.Kind().Regenerate(t.Context(), input(t, files, portEntry(), out.Entry), out.Entry)
	if err != nil {
		t.Fatal(err)
	}
	if len(regen.Appends) != 0 {
		t.Errorf("appends to gone files = %+v", regen.Appends)
	}
	want := []genkind.Decision{
		{Path: dir + "/client.go", Line: 1, Text: dir + "/client.go is gone: restore it, golaw never rewrites owned code"},
		{Path: dir + "/client_test.go", Line: 1, Text: dir + "/client_test.go is gone: restore it, golaw never rewrites owned code"},
	}
	if !slices.Equal(regen.Decisions, want) {
		t.Errorf("decisions = %+v, want %+v", regen.Decisions, want)
	}
}

func TestRegenerateKeepsARedTestOwnedCodeDeclares(t *testing.T) {
	t.Parallel()
	out := newAdapter(t, gentest.Contracts(), "Client")
	files := withPurge(written(gentest.Contracts(), out))
	files[dir+"/client_test.go"] = append(files[dir+"/client_test.go"], []byte("\nfunc TestClientPurgePlans(t *testing.T) { t.Parallel() }\n")...)
	regen, err := adapterhttp.Kind().Regenerate(t.Context(), input(t, files, portEntry(), out.Entry), out.Entry)
	if err != nil {
		t.Fatal(err)
	}
	if len(regen.Appends) != 1 || regen.Appends[0].Path != dir+"/client.go" {
		t.Errorf("appends = %+v, want the stub alone", regen.Appends)
	}
	if len(regen.Bindings) != 1 {
		t.Errorf("the regenerated adapter dropped its binding: %+v", regen.Bindings)
	}
}

func TestKindStopsOnACancelledContext(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	in := input(t, gentest.Contracts(), portEntry())
	if _, err := adapterhttp.Kind().New(ctx, in, request("Client")); !errors.Is(err, context.Canceled) {
		t.Errorf("New = %v", err)
	}
	if _, err := adapterhttp.Kind().Regenerate(ctx, in, manifest.Entry{}); !errors.Is(err, context.Canceled) {
		t.Errorf("Regenerate = %v", err)
	}
}

func TestKindRefusesWhatItCannotPlace(t *testing.T) {
	t.Parallel()
	noContext := gentest.Contracts()
	noContext[portsPath] = bytes.Replace(noContext[portsPath], []byte("ArchivePlan(ctx context.Context, account AccountID) error"),
		[]byte("ArchivePlan(account AccountID) error"), 1)
	if _, err := adapterhttp.Kind().New(t.Context(), input(t, gentest.Contracts()), request("Client")); !errors.Is(err, genkind.ErrRequest) {
		t.Errorf("New without the port entry = %v", err)
	}
	if _, err := adapterhttp.Kind().New(t.Context(), input(t, noContext, portEntry()), request("Client")); !errors.Is(err, genkind.ErrRequest) {
		t.Errorf("New over a method without a context = %v", err)
	}
	entry := manifest.Entry{Kind: genkind.KindAdapterHTTP, Template: "adapter-http@1", Name: "planapi.Client"}
	if _, err := adapterhttp.Kind().Regenerate(t.Context(), input(t, gentest.Contracts(), portEntry()), entry); !errors.Is(err, genkind.ErrRequest) {
		t.Errorf("Regenerate of an entry without a contract = %v", err)
	}
	broken := gentest.Contracts()
	broken[dir+"/broken.go"] = []byte("package planapi\nfunc {\n")
	if _, err := adapterhttp.Kind().New(t.Context(), input(t, broken, portEntry()), request("Client")); !errors.Is(err, genkind.ErrCannotJudge) {
		t.Errorf("New over a package that does not parse = %v", err)
	}
}

func TestALayoutBelowThePackageNamesTheImport(t *testing.T) {
	t.Parallel()
	in := input(t, gentest.Contracts(), portEntry())
	in.Policy.Layout.AdapterHTTP = "internal/clients/{package}/http"
	out, err := adapterhttp.Kind().New(t.Context(), in, request("Client"))
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Bindings) != 1 || out.Bindings[0].Import.Name != "planapi" {
		t.Errorf("bindings = %+v", out.Bindings)
	}
	suite := byPath(out.Regenerable)["internal/clients/planapi/http/zz_generated_client_contract_test.go"]
	if !bytes.Contains(suite, []byte(`planapi "example.com/slice/internal/clients/planapi/http"`)) {
		t.Errorf("the contract test does not name the adapter's import:\n%s", suite)
	}
}

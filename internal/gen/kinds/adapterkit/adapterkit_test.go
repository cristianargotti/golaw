package adapterkit_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/contract"
	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/kinds/adapterkit"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/gentest"
	"github.com/cristianargotti/golaw/internal/model"
)

const portsPath = "internal/application/growth/ports.go"

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

func adapterRequest(area, typ, port string) genkind.Request {
	req := genkind.Request{Name: genkind.Name{Area: area, Type: typ}}
	if portArea, portType, ok := strings.Cut(port, "."); ok {
		req.Port = genkind.Name{Area: portArea, Type: portType}
	}
	return req
}

// portsWith replaces the Ledger interface of the fixture with methods.
func portsWith(methods string) map[string][]byte {
	files := gentest.Contracts()
	files[portsPath] = []byte("// Package growth holds the plans of an account and the ports they need.\npackage growth\n\n" +
		"import (\n\t\"context\"\n\t\"time\"\n)\n\n// Ledger stores the plans of an account.\ntype Ledger interface {\n" + methods + "}\n")
	return files
}

func plansTarget(t *testing.T, files map[string][]byte) adapterkit.Target {
	t.Helper()
	in := input(t, files, portEntry())
	target, err := adapterkit.FromRequest(in, genkind.KindAdapterSQL, in.Policy.Layout.AdapterSQL, adapterRequest("postgres", "Plans", "growth.Ledger"))
	if err != nil {
		t.Fatal(err)
	}
	return target
}

func TestFromRequestResolvesThePortAndTheLayout(t *testing.T) {
	t.Parallel()
	got := plansTarget(t, gentest.Contracts())
	tests := map[string][2]string{
		"Dir":     {got.Dir, "internal/adapters/out/postgres"},
		"Package": {got.Package.Path, sliceModule + "/internal/adapters/out/postgres"},
		"PortPkg": {got.PortPkg.Path, sliceModule + "/internal/application/growth"},
		"Suite":   {got.Suite.Path, sliceModule + "/internal/application/growth/ledgertest"},
		"Recv":    {got.Recv, "p"},
		"Port":    {got.Port.Name, "Ledger"},
		"Lower":   {got.Lower(), "plans"},
		"Hole":    {got.Hole("ReadPlan"), "plans.read-plan"},
	}
	for name, pair := range tests {
		if pair[0] != pair[1] {
			t.Errorf("%s = %q, want %q", name, pair[0], pair[1])
		}
	}
	if got.Contract != portEntry().Contracts[0] {
		t.Errorf("Contract = %+v", got.Contract)
	}
}

func TestFromRequestRefusesWithTheFix(t *testing.T) {
	t.Parallel()
	existing := manifest.Entry{
		Kind: genkind.KindAdapterSQL, Template: "adapter-sql@1", Name: "postgres.Plans",
		Contracts:   []manifest.Contract{{Path: portsPath, Symbol: "Ledger"}},
		Regenerable: []string{"internal/adapters/out/postgres/zz_generated_plans.go"},
	}
	replica := existing
	replica.Name = "postgres.Replica"
	tests := []struct {
		name    string
		req     genkind.Request
		entries []manifest.Entry
		err     error
		text    string
	}{
		{"no port flag", adapterRequest("postgres", "Plans", ""), []manifest.Entry{portEntry()}, genkind.ErrRequest, "--port"},
		{"a port without its entry", adapterRequest("postgres", "Plans", "growth.Ledger"), nil, genkind.ErrRequest, "golaw new port growth.Ledger --usecase"},
		{"a package that is not a Go name", adapterRequest("post-gres", "Plans", "growth.Ledger"), []manifest.Entry{portEntry()}, genkind.ErrRequest, "post-gres"},
		{"a type that is not exported", adapterRequest("postgres", "plans", "growth.Ledger"), []manifest.Entry{portEntry()}, genkind.ErrRequest, "exported"},
		{"an instance that exists", adapterRequest("postgres", "Plans", "growth.Ledger"), []manifest.Entry{portEntry(), existing}, genkind.ErrExists, "internal/adapters/out/postgres/zz_generated_plans.go"},
		{"near: another instance of the package", adapterRequest("postgres", "Plans", "growth.Ledger"), []manifest.Entry{portEntry(), replica}, nil, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			in := input(t, gentest.Contracts(), tc.entries...)
			_, err := adapterkit.FromRequest(in, genkind.KindAdapterSQL, in.Policy.Layout.AdapterSQL, tc.req)
			if !errors.Is(err, tc.err) || (tc.err == nil) != (err == nil) {
				t.Fatalf("error = %v, want %v", err, tc.err)
			}
			if err != nil && !strings.Contains(err.Error(), tc.text) {
				t.Errorf("error %q does not name %q", err, tc.text)
			}
		})
	}
}

func TestMethodsQualifyThePortTypes(t *testing.T) {
	t.Parallel()
	methods, imports, err := adapterkit.Methods(plansTarget(t, gentest.Contracts()))
	if err != nil || len(methods) != 2 {
		t.Fatalf("Methods = %d, %v", len(methods), err)
	}
	read, archive := methods[0], methods[1]
	tests := map[string][2]string{
		"read signature":    {read.Signature(), "(ctx context.Context, account growth.AccountID) (growth.Plan, error)"},
		"read returns":      {read.Returns(), "zero, "},
		"read hole args":    {read.HoleArgs(), ", ctx, account"},
		"read discard":      {read.Discard(), "_, "},
		"read call":         {read.CallArgs(), "t.Context(), account"},
		"read action":       {read.Action, "actionPlansReadPlan"},
		"read phrase":       {read.Phrase, "postgres: plans read plan"},
		"read hole":         {read.Hole, "plans.read-plan"},
		"read test":         {read.Test("Plans"), "TestPlansReadPlan"},
		"read doc":          {strings.Join(read.Doc, "|"), "ReadPlan reads the plan of one account."},
		"read message":      {read.Message("call it"), `"hole plans.read-plan is open: call it"`},
		"archive signature": {archive.Signature(), "(ctx context.Context, account growth.AccountID) error"},
		"archive returns":   {archive.Returns(), ""},
		"archive discard":   {archive.Discard(), ""},
	}
	for name, pair := range tests {
		if pair[0] != pair[1] {
			t.Errorf("%s = %q, want %q", name, pair[0], pair[1])
		}
	}
	if !slices.Equal(read.Zeros, []genkind.Param{{Name: "zero", Type: "growth.Plan"}}) ||
		!slices.Equal(read.Inputs, []genkind.Param{{Name: "account", Type: "growth.AccountID"}}) {
		t.Errorf("zeros %+v, inputs %+v", read.Zeros, read.Inputs)
	}
	want := []genkind.Import{{Path: "context"}, {Path: sliceModule + "/internal/application/growth"}}
	if !slices.Equal(imports, want) {
		t.Errorf("imports = %+v, want %+v", imports, want)
	}
}

func TestMethodsNameAndQualifyEveryShape(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, methods, signature string
		err                      error
		text                     string
	}{
		{
			"composite types", "\tListPlans(ctx context.Context, accounts []AccountID, since time.Time, limit int) (map[AccountID]*Plan, error)\n",
			"(ctx context.Context, accounts []growth.AccountID, since time.Time, limit int) (map[growth.AccountID]*growth.Plan, error)", nil, "",
		},
		{
			"near: unnamed parameters get names", "\tReadPlan(context.Context, AccountID) (Plan, error)\n",
			"(ctx context.Context, arg1 growth.AccountID) (growth.Plan, error)", nil, "",
		},
		{"a method without an error", "\tCount(ctx context.Context) int\n", "", genkind.ErrRequest, "Count"},
		{"a method without a context", "\tReadPlan(account AccountID) (Plan, error)\n", "", genkind.ErrRequest, "context.Context"},
		{"an unexported type", "\tReadPlan(ctx context.Context, account accountID) (Plan, error)\n", "", genkind.ErrCannotJudge, "accountID"},
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

func TestUsesKeepsTheImportsAnExpressionNames(t *testing.T) {
	t.Parallel()
	imports := []genkind.Import{{Path: "context"}, {Path: "time"}, {Path: sliceModule + "/internal/application/growth"}, {Name: "v2", Path: "example.com/lib/v2"}}
	got := adapterkit.Uses(imports, []string{"[]growth.AccountID", "map[string]*v2.Item", "untimed.Value"})
	want := []genkind.Import{{Name: "v2", Path: "example.com/lib/v2"}, {Path: sliceModule + "/internal/application/growth"}}
	if !slices.Equal(got, want) {
		t.Errorf("Uses = %+v, want %+v", got, want)
	}
}

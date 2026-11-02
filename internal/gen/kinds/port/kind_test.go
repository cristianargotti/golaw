package port_test

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/kinds/port"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/gentest"
)

func TestTheKindIsPortAtTemplateOne(t *testing.T) {
	t.Parallel()
	if k := port.Kind(); k.Name() != genkind.KindPort || k.Template() != 1 {
		t.Errorf("Kind() = %s@%d", k.Name(), k.Template())
	}
}

func TestNewPortWritesItsPartsAndAsksForItsUseCase(t *testing.T) {
	t.Parallel()
	out, err := port.Kind().New(t.Context(), input(t, gentest.Contracts(), nil, ledger()), request("Ledger", "ReadPlan"))
	if err != nil {
		t.Fatal(err)
	}
	want := ledgerEntry()
	if !reflect.DeepEqual(out.Entry, want) {
		t.Errorf("entry = %+v, want %+v", out.Entry, want)
	}
	if got := paths(out.Regenerable); !slices.Equal(got, want.Regenerable) {
		t.Errorf("regenerable = %v", got)
	}
	if got := paths(out.Owned); !slices.Equal(got, want.Owned) {
		t.Errorf("owned = %v", got)
	}
	assertHas(t, content(t, out.Owned, ledgerDir+"cases.go"),
		"// Package ledgertest holds the fake of the port growth.Ledger and // the contract suite every implementation of it passes. package ledgertest",
		"// readPlanCases are the behavioral cases of ReadPlan every implementation of // growth.Ledger passes.",
		"func readPlanCases() []Case { // golaw:hole ledger.read-plan declare the inputs and the expected answer of ReadPlan for every implementation of growth.Ledger",
		"func archivePlanCases() []Case {",
		"return nil",
	)
	needs := []genkind.Need{{Kind: genkind.KindUsecase, Request: genkind.Request{
		Name: genkind.Name{Area: "growth", Type: "ReadPlan"}, Ports: []string{"Ledger"}, Port: genkind.Name{Area: "growth", Type: "Ledger"},
	}}}
	if !reflect.DeepEqual(out.Needs, needs) {
		t.Errorf("needs = %+v, want %+v", out.Needs, needs)
	}
	if len(out.Appends) != 0 || len(out.Decisions) != 0 || len(out.Bindings) != 0 {
		t.Errorf("appends %v, decisions %v, bindings %v", out.Appends, out.Decisions, out.Bindings)
	}
	assertLaws(t, out)
}

func TestNewPortJoinsAnExistingUseCase(t *testing.T) {
	t.Parallel()
	entries := []manifest.Entry{ledgerEntry(), readPlanEntry("Ledger")}
	out, err := port.Kind().New(t.Context(), input(t, gentest.Contracts(), entries, clock()), request("Clock", "ReadPlan"))
	if err != nil {
		t.Fatal(err)
	}
	want := []genkind.Need{{Kind: genkind.KindUsecase, Request: genkind.Request{
		Name: genkind.Name{Area: "growth", Type: "ReadPlan"}, Ports: []string{"Ledger", "Clock"}, In: "AccountID", Out: "Plan",
		Port: genkind.Name{Area: "growth", Type: "Clock"},
	}}}
	if !reflect.DeepEqual(out.Needs, want) {
		t.Errorf("needs = %+v, want %+v", out.Needs, want)
	}
}

func TestNewPortAskedByItsUseCaseAsksNothingBack(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		entries []manifest.Entry
		req     genkind.Request
	}{
		"red: a port a use case asked for": {req: genkind.Request{
			Name: genkind.Name{Area: "growth", Type: "Ledger"}, Usecase: "ReadPlan", Port: genkind.Name{Area: "growth", Type: "ReadPlan"},
		}},
		"near: a use case that already lists the port": {entries: []manifest.Entry{readPlanEntry("Ledger")}, req: request("Ledger", "ReadPlan")},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			out, err := port.Kind().New(t.Context(), input(t, gentest.Contracts(), tc.entries, ledger()), tc.req)
			if err != nil || len(out.Needs) != 0 {
				t.Errorf("New = %+v, %v, want no need", out.Needs, err)
			}
		})
	}
}

func TestNewPortOfAnUndeclaredInterfaceAppendsItsStub(t *testing.T) {
	t.Parallel()
	out, err := port.Kind().New(t.Context(), input(t, gentest.Contracts(), nil), request("Wallet", "Pay"))
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Appends) != 1 || out.Appends[0].Path != portsPath || !slices.Equal(out.Appends[0].Holes, []string{"wallet.methods"}) || len(out.Appends[0].Decls) != 1 {
		t.Fatalf("appends = %+v", out.Appends)
	}
	assertHas(t, string(out.Appends[0].Decls[0]),
		"// Wallet is a port of growth: declare its methods and the hook // regenerates its fake and its contract suite.",
		"type Wallet interface { // golaw:hole wallet.methods declare the methods of Wallet in internal/application/growth/ports.go",
	)
	if got := out.Entry.Contracts; !reflect.DeepEqual(got, []manifest.Contract{{Path: portsPath, Symbol: "Wallet"}}) {
		t.Errorf("contracts = %+v", got)
	}
	if got := paths(out.Owned); !slices.Equal(got, []string{area + "wallettest/cases.go"}) {
		t.Errorf("owned = %v", got)
	}
}

func TestNewPortInAnEmptyAreaWritesItsPortsFile(t *testing.T) {
	t.Parallel()
	req := genkind.Request{Name: genkind.Name{Area: "billing", Type: "Wallet"}, Usecase: "Pay"}
	out, err := port.Kind().New(t.Context(), input(t, gentest.Contracts(), nil), req)
	if err != nil {
		t.Fatal(err)
	}
	assertHas(t, content(t, out.Owned, "internal/application/billing/ports.go"),
		"// Package billing holds the use cases of billing and the ports they // consume.",
		"package billing",
		"type Wallet interface {",
	)
	if len(out.Appends) != 0 {
		t.Errorf("appends = %+v, want none: the file is new", out.Appends)
	}
}

func TestNewPortRefusesWhatItCannotGenerate(t *testing.T) {
	t.Parallel()
	stub := "package growth\n\n// Wallet is a stub.\ntype Wallet interface{}\n"
	tests := map[string]struct {
		files   map[string][]byte
		entries []manifest.Entry
		ports   []genkind.Port
		req     genkind.Request
		want    error
		text    string
	}{
		"red: no use case":                                       {req: request("Ledger", ""), want: genkind.ErrRequest, text: "golaw new port needs --usecase"},
		"red: an area that is not a package name":                {req: genkind.Request{Name: genkind.Name{Area: "Growth", Type: "Ledger"}, Usecase: "ReadPlan"}, want: genkind.ErrRequest, text: "is not <area>.<Type>"},
		"red: a type that is not exported":                       {req: request("ledger", "ReadPlan"), want: genkind.ErrRequest, text: "is not <area>.<Type>"},
		"red: a use case that is not a Go name":                  {req: request("Ledger", "read-plan"), want: genkind.ErrRequest, text: "--usecase read-plan"},
		"red: a port that has its entry":                         {entries: []manifest.Entry{ledgerEntry()}, req: request("Ledger", "ReadPlan"), want: genkind.ErrExists, text: ledgerDir + "zz_generated_fake.go"},
		"red: a second interface without methods":                {files: withFile(area+"wallet.go", stub), req: request("Purse", "Pay"), want: genkind.ErrRequest, text: "already declares the port Wallet without methods"},
		"red: a contract the reader cannot judge":                {req: request("Ledger", "ReadPlan"), want: genkind.ErrCannotJudge, text: "Ledger"},
		"red: an area file that does not parse":                  {files: withFile(area+"broken.go", "package growth\n\nfunc {\n"), req: request("Wallet", "Pay"), want: genkind.ErrCannotJudge, text: "broken.go does not parse"},
		"near: an area name with a digit":                        {req: genkind.Request{Name: genkind.Name{Area: "growth2", Type: "Wallet"}, Usecase: "Pay"}},
		"near: a stub beside ports with methods":                 {req: request("Wallet", "Pay")},
		"near: the stub of a port the same use case run creates": {files: withFile(area+"wallet.go", stub), req: genkind.Request{Name: genkind.Name{Area: "growth", Type: "Purse"}, Usecase: "Pay", Ports: []string{"Wallet", "Purse"}}},
		"near: the stub of the port it names":                    {files: withFile(area+"wallet.go", stub), ports: []genkind.Port{{Name: "Wallet", Package: "growth", Path: area + "wallet.go", Dir: strings.TrimSuffix(area, "/")}}, req: request("Wallet", "Pay")},
		"red: the methods of a port of another area": {
			files: withFile(billingPorts, "package billing\n\nimport \"time\"\n\n// Clock tells the time.\ntype Clock interface {\n\tNow() time.Time\n}\n"), entries: []manifest.Entry{growthClockEntry()},
			ports: []genkind.Port{clock(), billingClock("Now")}, req: genkind.Request{Name: genkind.Name{Area: "billing", Type: "Clock"}, Usecase: "Stamp"},
			want: genkind.ErrRequest, text: "billing.Clock has the methods of growth.Clock",
		},
		"near: a port of the same name with other methods": {
			files: withFile(billingPorts, "package billing\n\nimport \"time\"\n\n// Clock tells the time.\ntype Clock interface {\n\tToday() time.Time\n}\n"), entries: []manifest.Entry{growthClockEntry()},
			ports: []genkind.Port{clock(), billingClock("Today")}, req: genkind.Request{Name: genkind.Name{Area: "billing", Type: "Clock"}, Usecase: "Stamp"},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			files := tc.files
			if files == nil {
				files = gentest.Contracts()
			}
			_, err := port.Kind().New(t.Context(), input(t, files, tc.entries, tc.ports...), tc.req)
			if tc.want == nil {
				if err != nil {
					t.Errorf("New = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, tc.want) || !strings.Contains(err.Error(), tc.text) {
				t.Errorf("New = %v, want %v naming %q", err, tc.want, tc.text)
			}
		})
	}
}

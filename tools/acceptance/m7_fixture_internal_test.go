package main

import (
	"bytes"
	"maps"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/goldentest"
	"github.com/cristianargotti/golaw/internal/gen/holes"
	"github.com/cristianargotti/golaw/internal/gen/kinds"
	"github.com/cristianargotti/golaw/internal/gentest"
)

// m7BillingFiles are the files the fixture adds to the shared contracts.
func m7BillingFiles() []string {
	return []string{"internal/application/billing/ports.go", "internal/application/billing/rate.go"}
}

func TestM7FixtureIsTheSharedContracts(t *testing.T) {
	t.Parallel()
	repo, err := m7Fixture(testRoot(t), m7RepoDir)
	if err != nil {
		t.Fatal(err)
	}
	third, err := m7Fixture(testRoot(t), m7ThirdPartyDir)
	if err != nil {
		t.Fatal(err)
	}
	for path, want := range gentest.Contracts() {
		if path == "golaw.yml" {
			if _, found := repo[path]; found {
				t.Fatal("the fixture carries a policy: init must write it")
			}
			continue
		}
		if !bytes.Equal(m7Either(repo, third, path), want) {
			t.Fatalf("%s differs from gentest.Contracts", path)
		}
		delete(repo, path)
	}
	if extra := slices.Sorted(maps.Keys(repo)); !slices.Equal(extra, m7BillingFiles()) {
		t.Fatalf("the fixture adds %q", extra)
	}
	for path := range third {
		if !strings.Contains(path, "/sqlcgen/") && !strings.HasPrefix(path, "api/gen/") {
			t.Fatalf("third party file %s sits outside sqlcgen and api/gen", path)
		}
	}
}

// m7Either answers the file of path from repo, else from third.
func m7Either(repo, third map[string][]byte, path string) []byte {
	if got, inRepo := repo[path]; inRepo {
		return got
	}
	return third[path]
}

// m7GoldenCommands are the commands of m7Commands as the engine takes them.
func m7GoldenCommands() []goldentest.Command {
	name := func(area, typ string) genkind.Name { return genkind.Name{Area: area, Type: typ} }
	return []goldentest.Command{
		{Kind: genkind.KindUsecase, Request: genkind.Request{Name: name("growth", "ReadPlan"), Ports: []string{"Ledger"}, In: "AccountID", Out: "Plan"}},
		{Kind: genkind.KindPort, Request: genkind.Request{Name: name("growth", "Clock"), Usecase: "ReadPlan"}},
		{Kind: genkind.KindUsecase, Request: genkind.Request{Name: name("growth", "ArchivePlan"), Ports: []string{"Ledger"}, In: "AccountID", Out: "Plan"}},
		{Kind: genkind.KindAdapterSQL, Request: genkind.Request{Name: name("postgres", "Plans"), Port: name("growth", "Ledger")}},
		{Kind: genkind.KindUsecase, Request: genkind.Request{Name: name("billing", "ReadRate"), Ports: []string{"Rates"}, In: "AccountID", Out: "Rate"}},
		{Kind: genkind.KindAdapterHTTP, Request: genkind.Request{Name: name("rateapi", "Client"), Port: name("billing", "Rates")}},
		{Kind: genkind.KindHandler, Request: genkind.Request{Name: name("growthapi", ""), API: "api/openapi.yaml", Usecases: []string{"growth"}}},
	}
}

// m7RequestFlags renders a request as the flags golaw new takes.
func m7RequestFlags(r genkind.Request) []string {
	var out []string
	add := func(flag, value string) {
		if value != "" {
			out = append(out, flag, value)
		}
	}
	add("--port", strings.Trim(r.Port.String(), "."))
	add("--usecase", r.Usecase)
	add("--ports", strings.Join(r.Ports, ","))
	add("--in", r.In)
	add("--out", r.Out)
	add("--api", r.API)
	add("--usecases", strings.Join(r.Usecases, ","))
	return out
}

func TestM7CommandsAreTheGoldenCommands(t *testing.T) {
	t.Parallel()
	golden, commands := m7GoldenCommands(), m7Commands()
	if len(golden) != len(commands) {
		t.Fatalf("%d golden commands, %d commands", len(golden), len(commands))
	}
	kindsSeen := make(map[string]bool)
	for i, c := range commands {
		want := append([]string{"new", golden[i].Kind, strings.Trim(golden[i].Request.Name.String(), ".")}, m7RequestFlags(golden[i].Request)...)
		if !slices.Equal(c.args(), want) {
			t.Fatalf("command %d = %q, want %q", i, c.args(), want)
		}
		kindsSeen[c.kind] = true
	}
	for _, k := range []string{genkind.KindPort, genkind.KindAdapterSQL, genkind.KindAdapterHTTP, genkind.KindUsecase, genkind.KindHandler} {
		if !kindsSeen[k] {
			t.Fatalf("no command writes the kind %s", k)
		}
	}
}

// m7OpenIDs are the ids of the open holes of files.
func m7OpenIDs(t *testing.T, files map[string][]byte) []string {
	t.Helper()
	var ids []string
	for path, content := range files {
		if !strings.HasSuffix(path, ".go") {
			continue
		}
		found, err := holes.Find(path, content)
		if err != nil {
			t.Fatalf("find the holes of %s: %v", path, err)
		}
		for _, h := range found {
			ids = append(ids, h.ID)
		}
	}
	return slices.Sorted(slices.Values(ids))
}

// m7Go runs the go command of the running toolchain offline in dir.
func m7Go(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOPROXY=off", "GOTOOLCHAIN=local", "GOWORK=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// TestM7FillsCloseEveryHoleTheCommandsOpen renders the acceptance commands
// in process, proves the recorded fills name exactly the holes they open,
// then writes the fills and builds, vets and tests the result offline.
func TestM7FillsCloseEveryHoleTheCommandsOpen(t *testing.T) {
	t.Parallel()
	base := gentest.Contracts()
	repo, err := m7Fixture(testRoot(t), m7RepoDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range m7BillingFiles() {
		base[path] = repo[path]
	}
	report := goldentest.Run(t, goldentest.Slice{
		Name: "acceptance", Module: "example.com/slice", Base: base, Kinds: kinds.All(), Commands: m7GoldenCommands(),
	}, goldentest.StepLaws)
	written := maps.Clone(base)
	maps.Copy(written, report.Files)
	fills, err := m7Fills(testRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	want := make([]string, 0, len(fills))
	for _, f := range fills {
		want = append(want, f.ID)
	}
	if got := m7OpenIDs(t, written); !slices.Equal(got, slices.Sorted(slices.Values(want))) {
		t.Fatalf("open holes %q, recorded fills %q", got, slices.Sorted(slices.Values(want)))
	}
	dir := t.TempDir()
	if err := m6WriteFiles(dir, written); err != nil {
		t.Fatal(err)
	}
	if err := m7ApplyFills(dir, fills); err != nil {
		t.Fatal(err)
	}
	for _, step := range [][]string{{"build", "./..."}, {"vet", "./..."}, {"test", "./..."}} {
		m7Go(t, dir, step...)
	}
}

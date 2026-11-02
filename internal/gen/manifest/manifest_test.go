package manifest_test

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/manifest"
)

// sample is the manifest of M7.6.1 in the block form Encode writes.
const sample = `golaw-gen: 1
support:
  path: internal/gensupport
  template: support@1
entries:
  - kind: adapter-sql
    template: adapter-sql@1
    name: postgres.Plans
    contracts:
      - path: internal/application/growth/ports.go
        symbol: Ledger
      - path: db/queries/plans.sql
    regenerable:
      - internal/adapters/out/postgres/zz_generated_plans.go
      - internal/adapters/out/postgres/zz_generated_plans_contract_test.go
    owned:
      - internal/adapters/out/postgres/plans.go
      - internal/adapters/out/postgres/plans_test.go
  - kind: usecase
    template: usecase@1
    name: growth.ReadPlan
    ports:
      - Ledger
      - Clock
    in: ReadPlanInput
    out: Plan
    owned:
      - internal/application/growth/read_plan.go
      - internal/application/growth/read_plan_test.go
`

func TestDecodeReadsTheAddendumManifest(t *testing.T) {
	t.Parallel()
	m, err := manifest.Decode([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	if m.Schema != manifest.Schema || m.Support != (manifest.Support{Path: "internal/gensupport", Template: "support@1"}) {
		t.Errorf("head = %d, %+v", m.Schema, m.Support)
	}
	if len(m.Entries) != 2 || m.Entries[0].Name != "postgres.Plans" || m.Entries[1].Kind != "usecase" {
		t.Fatalf("entries = %+v", m.Entries)
	}
	if got := m.Entries[0].Contracts[0]; got != (manifest.Contract{Path: "internal/application/growth/ports.go", Symbol: "Ledger"}) {
		t.Errorf("first contract = %+v", got)
	}
	if got := m.Entries[1].Ports; !reflect.DeepEqual(got, []string{"Ledger", "Clock"}) {
		t.Errorf("ports = %v", got)
	}
	if manifest.Path != ".golaw/gen.yml" {
		t.Errorf("Path = %q", manifest.Path)
	}
}

func TestEncodeIsDeterministicAndRoundTrips(t *testing.T) {
	t.Parallel()
	m, err := manifest.Decode([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	first, err := m.Encode()
	if err != nil {
		t.Fatal(err)
	}
	second, err := m.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("two encodings of one manifest differ")
	}
	if !strings.HasPrefix(string(first), "golaw-gen: 1\nsupport:\n") || strings.Contains(string(first), "\r") {
		t.Errorf("encoding starts %q", string(first[:min(len(first), 40)]))
	}
	back, err := manifest.Decode(first)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back, m) {
		t.Errorf("Decode(Encode(m)) = %+v, want %+v", back, m)
	}
}

func TestDecodeRefusesWhatTheCLIWouldNeverWrite(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"an unknown key":               strings.Replace(sample, "    in: ReadPlanInput\n", "    in: ReadPlanInput\n    hash: abc\n", 1),
		"another schema":               strings.Replace(sample, "golaw-gen: 1", "golaw-gen: 2", 1),
		"an entry without a kind":      strings.Replace(sample, "  - kind: usecase\n", "  - kind: \"\"\n", 1),
		"an entry without a name":      strings.Replace(sample, "    name: growth.ReadPlan\n", "    name: \"\"\n", 1),
		"a name without a dot":         strings.Replace(sample, "name: growth.ReadPlan", "name: ReadPlan", 1),
		"a template of another kind":   strings.Replace(sample, "template: usecase@1", "template: port@1", 1),
		"a template without a version": strings.Replace(sample, "template: usecase@1", "template: usecase", 1),
		"entries out of order":         reorder(sample),
		"a duplicate entry":            sample + duplicate,
		"a path that leaves the root":  strings.Replace(sample, "internal/application/growth/read_plan.go", "../read_plan.go", 1),
		"an absolute path":             strings.Replace(sample, "path: db/queries/plans.sql", "path: /db/queries/plans.sql", 1),
		"a second document":            sample + "---\ngolaw-gen: 1\n",
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := manifest.Decode([]byte(data)); !errors.Is(err, manifest.ErrManifest) {
				t.Errorf("Decode = %v, want ErrManifest", err)
			}
		})
	}
}

func TestDecodeAcceptsTheNearMisses(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"no entries yet":             "golaw-gen: 1\nsupport:\n  path: internal/gensupport\n  template: support@1\nentries: []\n",
		"an entry without lists":     "golaw-gen: 1\nsupport:\n  path: internal/gensupport\n  template: support@1\nentries:\n  - kind: port\n    template: port@1\n    name: growth.Ledger\n",
		"a two digit template":       strings.Replace(sample, "template: usecase@1", "template: usecase@12", 1),
		"one kind, two names sorted": strings.Replace(sample, "  - kind: usecase\n", "  - kind: adapter-sql\n    template: adapter-sql@1\n    name: postgres.Users\n  - kind: usecase\n", 1),
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := manifest.Decode([]byte(data)); err != nil {
				t.Errorf("Decode = %v, want nil", err)
			}
		})
	}
}

// duplicate repeats the use case entry at the end, sorted but twice.
const duplicate = `  - kind: usecase
    template: usecase@1
    name: growth.ReadPlan
`

// reorder moves the use case entry before the adapter entry.
func reorder(text string) string {
	head, rest, _ := strings.Cut(text, "  - kind: adapter-sql\n")
	adapter, usecase, _ := strings.Cut(rest, "  - kind: usecase\n")
	return head + "  - kind: usecase\n" + usecase + "  - kind: adapter-sql\n" + adapter
}

func TestRefusalsNameTheKeyOrTheEntry(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		data string
		want string
	}{
		"an empty document":      {data: "", want: ".golaw/gen.yml is empty"},
		"a support template":     {data: strings.Replace(sample, "template: support@1", "template: support@0", 1), want: "support.template"},
		"a support path":         {data: strings.Replace(sample, "path: internal/gensupport", "path: internal/./gensupport", 1), want: "support.path"},
		"an unclean output path": {data: strings.Replace(sample, "internal/application/growth/read_plan.go", "internal/application//growth/read_plan.go", 1), want: "entries[1] growth.ReadPlan: path"},
		"a template of another kind": {
			data: strings.Replace(sample, "template: usecase@1", "template: port@1", 1),
			want: "entries[1] growth.ReadPlan: template port@1 is not of kind usecase",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := manifest.Decode([]byte(tc.data))
			if !errors.Is(err, manifest.ErrManifest) || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Decode = %v, want ErrManifest naming %q", err, tc.want)
			}
		})
	}
}

func TestEncodeRefusesAnInvalidManifest(t *testing.T) {
	t.Parallel()
	if _, err := (manifest.Manifest{Schema: 2}).Encode(); !errors.Is(err, manifest.ErrManifest) {
		t.Errorf("Encode = %v, want ErrManifest", err)
	}
}

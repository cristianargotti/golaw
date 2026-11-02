//go:build !race

package kinds_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/gen"
	"github.com/cristianargotti/golaw/internal/gen/contract"
	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/kinds"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/gentest"
	"github.com/cristianargotti/golaw/internal/model"
)

const (
	portsFile    = "internal/application/growth/ports.go"
	engineBudget = 100 * time.Millisecond
)

// longPorts pads the fixture's port contract with doc lines to the 300
// lines M7.7 measures, so the parse and the render see a real file size.
func longPorts(t *testing.T, ports []byte) []byte {
	t.Helper()
	text := string(ports)
	at := strings.Index(text, "// Ledger stores the plans of an account.")
	if at < 0 {
		t.Fatal("the fixture declares no Ledger port")
	}
	var doc strings.Builder
	for i := strings.Count(text, "\n"); i < 300; i++ {
		fmt.Fprintf(&doc, "// Note %d keeps the contract at the length the engine budget names.\n", i)
	}
	return []byte(text[:at] + doc.String() + "//\n" + text[at:])
}

func engineInput(t *testing.T, files map[string][]byte, m manifest.Manifest) gen.Input {
	t.Helper()
	snap := gentest.Snapshot(t, files)
	return gen.Input{
		Module: "example.com/slice", Version: "0.3.0", Policy: model.DefaultGenerate(),
		Snapshot: snap, Manifest: m, Contracts: contract.Reader{Snapshot: snap},
		Kinds: kinds.All(), Main: "api",
	}
}

// One contract of 300 lines regenerated in under 100 ms warm at the 95th
// percentile of 20 runs after two warm ups (M7.7, M7.13 row 3).
func TestGoldenEngineRegeneratesOneContractWithinItsBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("the engine budget is measured by make golden-gen")
	}
	t.Parallel()
	files := gentest.Contracts()
	files[portsFile] = longPorts(t, files[portsFile])
	if lines := strings.Count(string(files[portsFile]), "\n"); lines < 300 {
		t.Fatalf("the contract holds %d lines, under the 300 of M7.7", lines)
	}
	res, err := gen.New(t.Context(), engineInput(t, files, manifest.Manifest{Schema: manifest.Schema}), genkind.KindUsecase, genkind.Request{
		Name: genkind.Name{Area: "growth", Type: "ReadPlan"}, Ports: []string{"Ledger", "Clock"}, In: "AccountID", Out: "Plan",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range res.Writes {
		files[w.Path] = w.Content
	}
	in := engineInput(t, files, res.Manifest)
	in.Changed = []string{portsFile}
	samples := make([]time.Duration, 0, 20)
	for i := range 22 {
		start := time.Now()
		if _, err := gen.Generate(t.Context(), in); err != nil {
			t.Fatal(err)
		}
		if i >= 2 {
			samples = append(samples, time.Since(start))
		}
	}
	slices.Sort(samples)
	if p95 := samples[18]; p95 >= engineBudget {
		t.Errorf("one contract of 300 lines regenerated in %v at the 95th percentile of 20 warm runs, over the %v of M7.7", p95, engineBudget)
	}
}

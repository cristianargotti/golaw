package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
)

func countPrefix(lines []string, prefix string) int {
	n := 0
	for _, line := range lines {
		if strings.HasPrefix(line, prefix) {
			n++
		}
	}
	return n
}

func TestNewWritesAPortItsUseCaseTheSupportAndTheManifest(t *testing.T) {
	t.Parallel()
	ta, repo := genApp(t)
	if code := ta.run(t.Context(), []string{"new", "port", "growth.Ledger", "--usecase", "ReadPlan"}); code != exitClean {
		t.Fatalf("exit = %d: %s%s", code, ta.stdout.String(), ta.stderr.String())
	}
	lines := strings.Split(strings.TrimSpace(ta.stdout.String()), "\n")
	written, open := countPrefix(lines, "written "), countPrefix(lines, "hole open ")
	if lines[0] != fmt.Sprintf("golaw new port growth.Ledger: %d files written, %d holes open", written, open) || written == 0 || open == 0 {
		t.Fatalf("stdout = %s", ta.stdout.String())
	}
	for _, line := range lines {
		if rel, ok := strings.CutPrefix(line, "written "); ok {
			if _, err := os.Stat(filepath.Join(repo.Root, filepath.FromSlash(rel))); err != nil {
				t.Errorf("%s is listed and absent: %v", rel, err)
			}
		}
	}
	data, err := os.ReadFile(filepath.Join(repo.Root, filepath.FromSlash(manifest.Path)))
	if err != nil {
		t.Fatal(err)
	}
	m, err := manifest.Decode(data)
	_, port := m.Lookup(genkind.KindPort, "growth.Ledger")
	_, usecase := m.Lookup(genkind.KindUsecase, "growth.ReadPlan")
	if err != nil || !port || !usecase || m.Support.Path != "internal/gensupport" {
		t.Errorf("manifest = %+v, %v", m, err)
	}
	if !strings.Contains(ta.stderr.String(), "arquivo gerado ") || !strings.Contains(ta.stderr.String(), "lacuna aberta ") {
		t.Errorf("the person's pt-BR lines are missing: %q", ta.stderr.String())
	}
}

func TestNewRefusesAnExistingNameWithExitOne(t *testing.T) {
	t.Parallel()
	ta, _ := genApp(t)
	args := []string{"new", "port", "growth.Ledger", "--usecase", "ReadPlan"}
	if code := ta.run(t.Context(), args); code != exitClean {
		t.Fatalf("first exit = %d: %s", code, ta.stderr.String())
	}
	ta.stdout.Reset()
	if code := ta.run(t.Context(), args); code != exitFindings || !strings.Contains(ta.stdout.String(), "refused") || !strings.Contains(ta.stdout.String(), manifest.Path) {
		t.Errorf("second exit = %d, stdout %q", code, ta.stdout.String())
	}
}

func TestNewDryRunPrintsTheDiffAndWritesNothing(t *testing.T) {
	t.Parallel()
	ta, repo := genApp(t)
	if code := ta.run(t.Context(), []string{"new", "port", "growth.Ledger", "--usecase", "ReadPlan", "--dry-run"}); code != exitClean {
		t.Fatalf("exit = %d: %s", code, ta.stderr.String())
	}
	out := ta.stdout.String()
	if !strings.Contains(out, "golaw new port growth.Ledger (dry run): ") || !strings.Contains(out, "--- internal/gensupport/zz_generated_hole.go") {
		t.Errorf("stdout = %s", out)
	}
	if status := repo.Git("status", "--porcelain"); status != "" {
		t.Errorf("a dry run wrote: %s", status)
	}
}

func TestNewWritesTheJSONReport(t *testing.T) {
	t.Parallel()
	ta, _ := genApp(t)
	if code := ta.run(t.Context(), []string{"new", "port", "growth.Ledger", "--usecase", "ReadPlan", "--json"}); code != exitClean {
		t.Fatalf("exit = %d: %s", code, ta.stderr.String())
	}
	var doc struct {
		Kind    string     `json:"kind"`
		Written []string   `json:"written"`
		Holes   []holeJSON `json:"holes"`
	}
	if err := json.Unmarshal(ta.stdout.Bytes(), &doc); err != nil || doc.Kind != genkind.KindPort || len(doc.Written) == 0 || len(doc.Holes) == 0 {
		t.Errorf("JSON = %+v, %v: %s", doc, err, ta.stdout.String())
	}
	if ta.stderr.Len() != 0 {
		t.Errorf("--json printed person lines: %q", ta.stderr.String())
	}
}

// Red rows: what golaw new refuses with exit 2 before it plans anything.
func TestNewRefusesAMalformedRequestWithExitTwo(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		args []string
		want string
	}{
		"a name of three parts": {[]string{"new", "port", "growth.Led.ger", "--usecase", "ReadPlan"}, "growth.Led.ger"},
		"an unknown kind":       {[]string{"new", "queue", "growth.Events"}, "unknown generator kind"},
		"a malformed port flag": {[]string{"new", "adapter-sql", "postgres.Plans", "--port", "growth."}, "growth."},
		"too many arguments":    {[]string{"new", "port", "growth.Ledger", "extra"}, "adapter"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ta, _ := genApp(t)
			if code := ta.run(t.Context(), tc.args); code != exitFailure || !strings.Contains(ta.stderr.String(), tc.want) {
				t.Errorf("exit = %d, stderr %q, want 2 naming %q", code, ta.stderr.String(), tc.want)
			}
		})
	}
}

func TestNewRequestReadsTheAddendumSpellingAndTheFlags(t *testing.T) {
	t.Parallel()
	kind, req, err := newRequest([]string{"adapter", "sql", "postgres.Plans"}, newOptions{port: "growth.Ledger"})
	want := genkind.Request{Name: genkind.Name{Area: "postgres", Type: "Plans"}, Port: genkind.Name{Area: "growth", Type: "Ledger"}}
	if err != nil || kind != genkind.KindAdapterSQL || req.Name != want.Name || req.Port != want.Port {
		t.Errorf("newRequest = %s, %+v, %v", kind, req, err)
	}
	kind, req, err = newRequest([]string{"handler", "plansapi"}, newOptions{api: "api/openapi.yaml", usecases: []string{"growth"}})
	if err != nil || kind != genkind.KindHandler || req.Name != (genkind.Name{Area: "plansapi"}) || req.API != "api/openapi.yaml" || len(req.Usecases) != 1 {
		t.Errorf("newRequest(handler) = %s, %+v, %v", kind, req, err)
	}
	for _, bad := range []string{"", ".Plans", "growth.", "a.b.c", "growth plans"} {
		if _, err := parseName(bad); !errors.Is(err, genkind.ErrRequest) {
			t.Errorf("parseName(%q) = %v, want ErrRequest", bad, err)
		}
	}
}

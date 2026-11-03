package main

import (
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/gen/thirdparty"
	"github.com/cristianargotti/golaw/internal/gentest"
	"github.com/cristianargotti/golaw/internal/proof"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/runnertest"
	"github.com/cristianargotti/golaw/internal/source"
)

var sqlcConfig = []byte("version: \"2\"\n")

// slowMachineToolLimit lets a test that proves what a generator run does,
// not how fast it starts, pass on a loaded runner whose process start
// alone can exceed the hook's limit.
const slowMachineToolLimit = 30 * time.Second

func sqlcGenerator() thirdparty.Generator {
	return thirdparty.Generator{
		Name: "sqlc", Config: "sqlc.yaml", Args: []string{"generate", "-f", "sqlc.yaml"},
		Inputs: []string{"db/queries/*.sql"}, Outputs: []string{"internal/adapters/out/postgres/sqlcgen"}, Hook: true,
	}
}

// warmGenerators installs the fake generators and starts each once, so the
// first start of a fresh executable never counts against the tool budget.
func warmGenerators(t *testing.T, h hookBinding, b runnertest.Behavior) string {
	t.Helper()
	paths := gentest.InstallGenerators(t, h.dirs.Tools, runnertest.Behavior{})
	if _, err := runner.Run(t.Context(), runner.Command{Name: paths["sqlc"]}); err != nil {
		t.Fatal(err)
	}
	paths = gentest.InstallGenerators(t, h.dirs.Tools, b)
	return filepath.Dir(paths["sqlc"])
}

func TestTheHookRunsSqlcUnderItsManagedConfiguration(t *testing.T) {
	t.Parallel()
	h := regenBinding(t)
	h.toolLimit = slowMachineToolLimit
	dir := warmGenerators(t, h, runnertest.Behavior{Stdout: "ok"})
	root := t.TempDir()
	snap := proof.NewTree(map[string][]byte{"sqlc.yaml": sqlcConfig})
	out := h.thirdParty(t.Context(), root, map[string]string{"sqlc.yaml": source.ContentHash(sqlcConfig)}, snap, []thirdparty.Generator{sqlcGenerator()})
	if !out.ran || len(out.causes) != 0 {
		t.Fatalf("out = %+v", out)
	}
	calls := runnertest.Calls(t, dir, "sqlc")
	if len(calls) == 0 || !slices.Equal(calls[len(calls)-1], []string{"generate", "-f", "sqlc.yaml"}) {
		t.Fatalf("calls = %q", calls)
	}
}

func TestTheHookLeavesTheOutputToGolawGenWhenItCannotRunTheGenerator(t *testing.T) {
	t.Parallel()
	snap := proof.NewTree(map[string][]byte{"sqlc.yaml": sqlcConfig})
	matching := map[string]string{"sqlc.yaml": source.ContentHash(sqlcConfig)}
	tests := map[string]struct {
		behavior *runnertest.Behavior
		managed  map[string]string
		cause    string
	}{
		"red: a generator past the tool budget":          {&runnertest.Behavior{Delay: "2s"}, matching, "sqlc passed the hook's 1.5 s limit"},
		"red: a generator that fails":                    {&runnertest.Behavior{Code: 1, Stderr: "error parsing queries\n"}, matching, "sqlc exited 1: error parsing queries"},
		"red: a configuration golaw.yml does not record": {&runnertest.Behavior{}, map[string]string{"sqlc.yaml": source.ContentHash([]byte("other"))}, "sqlc.yaml differs from the managed configuration golaw.yml records"},
		"red: a generator setup never built":             {nil, matching, "sqlc 1.31.1 is not built on this machine: run golaw setup --gen"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := regenBinding(t)
			// Only the budget row judges the hook's own limit; the others
			// prove a cause, which a slow machine must not turn into a timeout.
			if tc.behavior == nil || tc.behavior.Delay == "" {
				h.toolLimit = slowMachineToolLimit
			}
			if tc.behavior != nil {
				warmGenerators(t, h, *tc.behavior)
			}
			out := h.thirdParty(t.Context(), t.TempDir(), tc.managed, snap, []thirdparty.Generator{sqlcGenerator()})
			if out.ran || !slices.Equal(out.causes, []string{tc.cause}) {
				t.Fatalf("out = %+v, want the cause %q", out, tc.cause)
			}
		})
	}
	h := regenBinding(t)
	out := h.thirdParty(t.Context(), t.TempDir(), nil, snap, []thirdparty.Generator{{Name: "goverter", Hook: false}})
	if out.ran || !slices.Equal(out.outside, []string{"goverter"}) || len(out.causes) != 0 {
		t.Fatalf("goverter = %+v", out)
	}
}

func TestManagedHashesReadThePolicyInForce(t *testing.T) {
	t.Parallel()
	files := gentest.Contracts()
	hash := source.ContentHash(sqlcConfig)
	files["golaw.yml"] = append(files["golaw.yml"], []byte("managed:\n  - { path: sqlc.yaml, sha256: "+hash+" }\n")...)
	files["sqlc.yaml"] = sqlcConfig
	repo := gentest.Module(t, files)
	got, err := regenBinding(t).managedHashes(t.Context(), repo.Root)
	if err != nil || got["sqlc.yaml"] != hash {
		t.Fatalf("managed = %v, %v", got, err)
	}
}

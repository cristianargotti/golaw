package thirdparty_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/thirdparty"
	"github.com/cristianargotti/golaw/internal/runnertest"
)

var sqlc = thirdparty.Generator{
	Name: "sqlc", Config: "sqlc.yaml", Args: []string{"generate", "-f", "sqlc.yaml"},
	Inputs: []string{"db/queries/*.sql", "db/migrations/*.sql"}, Outputs: []string{"internal/adapters/out/postgres/sqlcgen"}, Hook: true,
}

// toolsRoot is a repository root holding the tools module.
func toolsRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "tools"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "tools", "go.mod"), []byte("module example.com/slice/tools\n\ngo 1.27\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestResolveAsksGoToolForTheBinary(t *testing.T) {
	t.Parallel()
	bin := t.TempDir()
	fake := runnertest.Install(t, bin, "sqlc", runnertest.Behavior{})
	goBin := runnertest.Install(t, bin, "go", runnertest.Behavior{Stdout: fake + "\n"})
	got, err := thirdparty.Resolve(t.Context(), goBin, toolsRoot(t), sqlc, nil)
	if err != nil || got != fake {
		t.Fatalf("Resolve = %q, %v, want %q", got, err, fake)
	}
	want := [][]string{{"tool", "-modfile=tools/go.mod", "-n", "sqlc"}}
	if calls := runnertest.Calls(t, bin, "go"); !reflect.DeepEqual(calls, want) {
		t.Errorf("go calls = %v, want %v", calls, want)
	}
}

// Red rows: every way the tools module cannot provide the generator is
// ErrMissing with its cause, never a silent pass.
func TestResolveNamesWhyAGeneratorIsMissing(t *testing.T) {
	t.Parallel()
	bin := t.TempDir()
	failing := runnertest.Install(t, bin, "go", runnertest.Behavior{Stderr: "no tool sqlc in tools/go.mod", Code: 1})
	tests := map[string]struct {
		goBin, root, want string
	}{
		"no tools module":     {failing, t.TempDir(), "tools/go.mod is absent"},
		"go answers an error": {failing, toolsRoot(t), "no tool sqlc"},
		"go cannot start":     {filepath.Join(bin, "absent-go"), toolsRoot(t), "go"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := thirdparty.Resolve(t.Context(), tc.goBin, tc.root, sqlc, nil)
			if !errors.Is(err, thirdparty.ErrMissing) || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "sqlc") {
				t.Errorf("Resolve = %v, want ErrMissing naming %q", err, tc.want)
			}
		})
	}
	empty := runnertest.Install(t, t.TempDir(), "go", runnertest.Behavior{Stdout: "\n"})
	if _, err := thirdparty.Resolve(t.Context(), empty, toolsRoot(t), sqlc, nil); !errors.Is(err, thirdparty.ErrMissing) {
		t.Errorf("an empty answer = %v, want ErrMissing", err)
	}
}

// Red row: a tools module init wrote without its go.sum names the command
// that completes it, not the go download hint, which run at the repository
// root writes no tools/go.sum. Near row: any other go tool error keeps its
// own text.
func TestResolveNamesTheTidyOfAnIncompleteToolsModule(t *testing.T) {
	t.Parallel()
	const missingSum = "go: github.com/sqlc-dev/sqlc/cmd/sqlc: missing go.sum entry for module providing package github.com/sqlc-dev/sqlc/cmd/sqlc; to add:\n\tgo mod download github.com/sqlc-dev/sqlc"
	for name, tc := range map[string]struct {
		stderr string
		tidy   bool
	}{
		"red: a missing go.sum entry":   {stderr: missingSum, tidy: true},
		"red: go.mod updates needed":    {stderr: "go: updates to go.mod needed; to update it:\n\tgo mod tidy", tidy: true},
		"near: another go tool failure": {stderr: "go: no such tool \"sqlc\""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			goBin := runnertest.Install(t, t.TempDir(), "go", runnertest.Behavior{Stderr: tc.stderr, Code: 1})
			_, err := thirdparty.Resolve(t.Context(), goBin, toolsRoot(t), sqlc, nil)
			text := fmt.Sprint(err)
			named := strings.Contains(text, "run go -C tools mod tidy and commit tools/go.mod and tools/go.sum")
			if !errors.Is(err, thirdparty.ErrMissing) || named != tc.tidy || strings.Contains(text, "to add") {
				t.Errorf("Resolve = %v, want ErrMissing, the tidy named %v and no download hint", err, tc.tidy)
			}
		})
	}
}

func TestExecRunsTheGeneratorArgumentsInTheDirectory(t *testing.T) {
	t.Parallel()
	bin := t.TempDir()
	fake := runnertest.Install(t, bin, "sqlc", runnertest.Behavior{Stdout: "ok"})
	out, err := thirdparty.Exec(t.Context(), fake, t.TempDir(), sqlc, nil)
	if err != nil || string(out.Stdout) != "ok" {
		t.Fatalf("Exec = %+v, %v", out, err)
	}
	if calls := runnertest.Calls(t, bin, "sqlc"); !reflect.DeepEqual(calls, [][]string{sqlc.Args}) {
		t.Errorf("sqlc calls = %v", calls)
	}
	broken := runnertest.Install(t, bin, "sqlc", runnertest.Behavior{Stderr: "plans.sql:2: syntax error", Code: 1})
	if _, err := thirdparty.Exec(t.Context(), broken, t.TempDir(), sqlc, nil); !errors.Is(err, thirdparty.ErrFailed) || !strings.Contains(err.Error(), "plans.sql:2: syntax error") {
		t.Errorf("a failing generator = %v, want ErrFailed with its output", err)
	}
	if _, err := thirdparty.Exec(t.Context(), filepath.Join(bin, "absent"), t.TempDir(), sqlc, nil); !errors.Is(err, thirdparty.ErrMissing) {
		t.Errorf("an absent binary = %v, want ErrMissing", err)
	}
}

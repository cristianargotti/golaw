package lint

import (
	"os"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

// TestRuffOnTheBytesNamesNoNamespacePackage: the post hook and the armed
// pre-commit rung feed ruff the bytes on stdin from the tool directory, so
// ruff cannot see the __init__.py beside the file and INP001 would be a
// false finding on every module of a package; it stays out of that run,
// while the other strict rules still judge the bytes.
func TestRuffOnTheBytesNamesNoNamespacePackage(t *testing.T) {
	t.Parallel()
	d := pythonDriver(t, FormatRuff)
	tool, ok := toolchain.Lookup(d.Tool)
	if !ok {
		t.Fatalf("toolchain.yml holds no %s", d.Tool)
	}
	resolved := testResolver(t).Full(t.Context(), tool)
	if !resolved.Usable() {
		t.Skipf("%s %s not installed: run golaw setup --surface", tool.Name, tool.Version)
	}
	dir := pythonMaterialize(t, d, model.SurfaceStrict)
	for _, tc := range []struct {
		name, source string
		want         map[string]int
	}{
		{"near a clean module of a package", "\"\"\"Deploy.\"\"\"\n\nVERSION = 2\n", map[string]int{}},
		{"red a bare except still judged", "\"\"\"Deploy.\"\"\"\n\ntry:\n    VERSION = 2\nexcept:\n    VERSION = 1\n", map[string]int{"e722": 1}},
	} {
		args := d.HookArgs(HookInput{Path: "deploy/p2.py", ConfigDir: dir})
		req := toolchain.Request{Path: resolved.Path, Args: args, Stdin: []byte(tc.source), Dir: dir, Env: toolchain.ToolEnv(os.Environ()), Timeout: pythonLintTimeout}
		ex, err := toolchain.Exec(t.Context(), req)
		if err != nil || ex.Killed {
			t.Fatalf("%s: run ruff: killed %v, %v", tc.name, ex.Killed, err)
		}
		issues, err := Parse(d.Format, d.Tool, ex.Output, dir)
		if err != nil {
			t.Fatalf("%s: parse: %v", tc.name, err)
		}
		got := pythonCodes(issues)
		delete(got, "ble001")
		delete(got, "s110")
		if len(got) != len(tc.want) || got["e722"] != tc.want["e722"] {
			t.Errorf("%s: codes = %v, want %v", tc.name, got, tc.want)
		}
	}
}

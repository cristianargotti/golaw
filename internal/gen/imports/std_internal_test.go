package imports

import (
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// TestStdListIsTheStandardLibraryOfTheToolchain holds the embedded list to
// go list std of the kit's Go line, internal and vendor packages left out,
// so a toolchain bump that forgets the list fails here.
func TestStdListIsTheStandardLibraryOfTheToolchain(t *testing.T) {
	t.Parallel()
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go is not on PATH: the std list is checked where the kit builds")
	}
	version, err := exec.CommandContext(t.Context(), goBin, "env", "GOVERSION").Output()
	if err != nil || !strings.HasPrefix(string(version), "go1.27") {
		t.Skipf("the std list is recorded from go 1.27, the kit's go line; this toolchain answers %q", version)
	}
	cmd := exec.CommandContext(t.Context(), goBin, "list", "-f", "{{.ImportPath}} {{.Name}}", "std")
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOFLAGS=")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for line := range strings.Lines(string(out)) {
		line = strings.TrimSpace(line)
		importPath, _, _ := strings.Cut(line, " ")
		if line == "" || strings.HasPrefix(importPath, "vendor/") || slices.Contains(strings.Split(importPath, "/"), "internal") {
			continue
		}
		want = append(want, line)
	}
	if got := strings.Split(strings.TrimSpace(stdList), "\n"); !slices.Equal(got, want) {
		t.Errorf("std.txt holds %d lines, go list std answers %d: regenerate it with the command of AGT-4", len(got), len(want))
	}
}

package support_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/kinds/support"
)

// goCommand runs the toolchain of the test itself, offline: the rendered
// module requires nothing outside the standard library. The go command puts
// its own bin directory first on the PATH of a test, so the lookup finds it.
func goCommand(t *testing.T, dir string, args ...string) string {
	t.Helper()
	name, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOTOOLCHAIN=local", "GOPROXY=off", "GOWORK=off")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func TestTheSupportPackageBuildsAndItsTestsCoverEveryLine(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a module with the go command; skipped under -short")
	}
	t.Parallel()
	out, err := support.Kind().New(t.Context(), input(), genkind.Request{})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	files := map[string][]byte{"go.mod": []byte("module example.com/slice\n\ngo 1.27\n")}
	for _, f := range out.Regenerable {
		files[f.Path] = f.Content
	}
	for rel, content := range files {
		target := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	goCommand(t, dir, "vet", "./...")
	tested := goCommand(t, dir, "test", "-count=1", "-cover", "./...")
	for line := range strings.SplitSeq(strings.TrimSpace(tested), "\n") {
		if strings.HasPrefix(line, "ok") && !strings.Contains(line, "coverage: 100.0% of statements") {
			t.Errorf("a support package is not covered line for line: %s", line)
		}
	}
	if !strings.Contains(tested, "example.com/slice/internal/gensupport") {
		t.Errorf("go test did not test the support package:\n%s", tested)
	}
}

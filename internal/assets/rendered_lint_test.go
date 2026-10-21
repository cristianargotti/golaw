package assets_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/runner"
)

// TestRenderedRepositoryLintsAndTidiesClean runs the pinned golangci-lint
// and go mod tidy -diff over the core templates as init renders them beside
// a module with one clean Go file: the strict configuration loads and finds
// nothing, and the module graph asks for no requirement the adopter lacks,
// so make lint and the deadcode asset are green the day init writes them.
// golangci-lint takes a lock in the temporary directory that every instance
// on the machine shares and gives up on it after five seconds, so the test
// allows parallel runners and an editor or another checkout cannot fail it.
func TestRenderedRepositoryLintsAndTidiesClean(t *testing.T) {
	t.Parallel()
	tool, ok := runner.LookupTool("golangci-lint")
	if !ok {
		t.Fatal("golangci-lint is not a pinned tool")
	}
	env := append((&exec.Cmd{}).Environ(), "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "GOWORK=off")
	linter, err := runner.Verify(t.Context(), tool, env)
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("CI installs golangci-lint %s before the tests: %v", tool.Version, err)
		}
		t.Skipf("golangci-lint %s is not on PATH, which make ci installs: %v", tool.Version, err)
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	files := renderedRepository(t, []string{"go"})
	files["main.go"] = []byte("// Command service is the module the core templates are rendered for.\npackage main\n\nfunc main() {}\n")
	for relative, content := range files {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// The dsl the rules file imports comes from the module cache, which the
	// kit's own go.mod already fills, or from the proxy on a fresh machine.
	online := append((&exec.Cmd{}).Environ(), "GOTOOLCHAIN=local", "GOWORK=off")
	for _, c := range []runner.Command{
		{Name: "go", Args: []string{"mod", "download", "github.com/quasilyte/go-ruleguard/dsl"}, Dir: root, Env: online},
		{Name: "go", Args: []string{"mod", "tidy", "-diff"}, Dir: root, Env: env},
		{Name: linter, Args: []string{"run", "--allow-parallel-runners", "./..."}, Dir: root, Env: env},
	} {
		out, err := runner.Run(t.Context(), c)
		if err != nil || out.Code != 0 {
			t.Errorf("%s %v = %d: %s%s: %v", c.Name, c.Args, out.Code, out.Stdout, out.Stderr, err)
		}
	}
}

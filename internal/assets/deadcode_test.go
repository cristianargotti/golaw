package assets_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/runner"
)

func TestDeadcodeAssetDetectsOnlyUnusedModuleRequirements(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, main string
		want       int
	}{
		{"red", "package main\n\nfunc main() {}\n", 1},
		{"near", "package main\n\nimport \"example.test/unused\"\n\nfunc main() { _ = unused.Value }\n", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			file := renderedAsset(t, "test/deadcode/reachability_test.go", assetContext())
			if file.Template.Ownership != assets.OwnedByDeveloper {
				t.Fatal("deadcode test is not developer-owned")
			}
			fixtures := map[string]string{
				"go.mod":           "module example.test/service\n\ngo 1.27\n\nrequire example.test/unused v1.0.0\n\nreplace example.test/unused => ./unused\n",
				"main.go":          tc.main,
				"unused/go.mod":    "module example.test/unused\n\ngo 1.27\n",
				"unused/unused.go": "package unused\n\nconst Value = 1\n",
				file.Path:          string(file.Data),
			}
			writeFixtures(t, root, fixtures)
			env := append((&exec.Cmd{}).Environ(),
				"GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "GOWORK=off")
			out, err := runner.Run(t.Context(), runner.Command{
				Name: "go", Dir: root, Env: env,
				Args: []string{"test", "-tags=deadcode", "./test/deadcode/"},
			})
			if err != nil || out.Code != tc.want {
				t.Fatalf("deadcode code = %d: %s: %s: %v", out.Code, out.Stdout, out.Stderr, err)
			}
		})
	}
}

// writeFixtures writes each fixture file beneath root, its directories
// first.
func writeFixtures(t *testing.T, root string, fixtures map[string]string) {
	t.Helper()
	for relative, content := range fixtures {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

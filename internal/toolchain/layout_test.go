package toolchain_test

import (
	"path/filepath"
	"regexp"
	"testing"

	"github.com/cristianargotti/golaw/internal/toolchain"
)

func mustLookup(t *testing.T, name string) toolchain.Tool {
	t.Helper()
	tool, ok := toolchain.Lookup(name)
	if !ok {
		t.Fatalf("no pinned %s", name)
	}
	return tool
}

func TestLayoutPathsFollowTheToolDirectoryLayout(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("home", "data", "golaw", "tools")
	ruff, eslint, yamllint := mustLookup(t, "ruff"), mustLookup(t, "eslint"), mustLookup(t, "yamllint")
	node, python := toolchain.NodeDir(dir), toolchain.PythonDir(dir)
	tests := map[string]struct{ got, want string }{
		"binary":           {toolchain.BinPath(dir, ruff, "darwin", "arm64"), filepath.Join(dir, "bin", "ruff", "0.17.1", "darwin-arm64", "ruff")},
		"windows binary":   {toolchain.BinPath(dir, ruff, "windows", "amd64"), filepath.Join(dir, "bin", "ruff", "0.17.1", "windows-amd64", "ruff.exe")},
		"released config":  {toolchain.ConfigDir(dir, "0.3.0", "strict"), filepath.Join(dir, "config", "0.3.0", "strict")},
		"dev build config": {toolchain.ConfigDir(dir, "", "parity"), filepath.Join(dir, "config", "devel", "parity")},
		"exec of binary":   {toolchain.ExecPath(dir, ruff, "linux", "amd64"), toolchain.BinPath(dir, ruff, "linux", "amd64")},
		"exec of npm":      {toolchain.ExecPath(dir, eslint, "linux", "amd64"), filepath.Join(node, "node_modules", ".bin", "eslint")},
		"npm on windows":   {toolchain.ExecPath(dir, eslint, "windows", "amd64"), filepath.Join(node, "node_modules", ".bin", "eslint.cmd")},
		"exec of pypi":     {toolchain.ExecPath(dir, yamllint, "darwin", "arm64"), filepath.Join(python, "bin", "yamllint")},
		"pypi on windows":  {toolchain.ExecPath(dir, yamllint, "windows", "arm64"), filepath.Join(python, "Scripts", "yamllint.exe")},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if tc.got != tc.want {
				t.Errorf("got %q, want %q", tc.got, tc.want)
			}
		})
	}
}

func TestNodeAndPythonDirectoriesAreNamedByTheirLock(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("data", "tools")
	id := regexp.MustCompile(`^[0-9a-f]{16}$`)
	node, python := toolchain.NodeDir(dir), toolchain.PythonDir(dir)
	if filepath.Dir(node) != filepath.Join(dir, "node") || !id.MatchString(filepath.Base(node)) {
		t.Errorf("NodeDir = %q", node)
	}
	venvParent := filepath.Dir(python)
	if filepath.Base(python) != "venv" || filepath.Dir(venvParent) != filepath.Join(dir, "python") || !id.MatchString(filepath.Base(venvParent)) {
		t.Errorf("PythonDir = %q", python)
	}
	if filepath.Base(node) == filepath.Base(venvParent) {
		t.Error("the npm and PyPI locks share one id")
	}
}

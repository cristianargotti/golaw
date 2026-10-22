package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func cinHome(t *testing.T) string {
	t.Helper()
	home, err := filepath.EvalSymlinks(t.TempDir())
	must(t, err)
	for _, entry := range cinEnvironment(home) {
		_, path, _ := strings.Cut(entry, "=")
		must(t, os.MkdirAll(path, 0o700))
	}
	return home
}

func cinEnvironment(home string) []string {
	return []string{
		"HOME=" + home,
		"USERPROFILE=" + home,
		"CODEX_HOME=" + filepath.Join(home, ".codex"),
		"CLAUDE_CONFIG_DIR=" + filepath.Join(home, ".claude"),
		"APPDATA=" + filepath.Join(home, "appdata"),
		"LOCALAPPDATA=" + filepath.Join(home, "localappdata"),
		"XDG_CONFIG_HOME=" + filepath.Join(home, "xdg-config"),
		"XDG_STATE_HOME=" + filepath.Join(home, "xdg-state"),
		"XDG_DATA_HOME=" + filepath.Join(home, "xdg-data"),
		"XDG_CACHE_HOME=" + filepath.Join(home, "xdg-cache"),
		"XDG_RUNTIME_DIR=" + filepath.Join(home, "xdg-runtime"),
		"XDG_CONFIG_DIRS=" + filepath.Join(home, "xdg-config-dirs"),
		"XDG_DATA_DIRS=" + filepath.Join(home, "xdg-data-dirs"),
	}
}

func cinWrite(t *testing.T, path string, data []byte) { //nolint:unused // the rules tests of the next task write files through it
	t.Helper()
	must(t, os.MkdirAll(filepath.Dir(path), 0o700))
	must(t, os.WriteFile(path, data, 0o600))
}

func cinRead(t *testing.T, path string) []byte { //nolint:unused // the installer tests of later tasks read files through it
	t.Helper()
	data, err := os.ReadFile(path)
	must(t, err)
	return data
}

package install

import (
	"os"
	"path/filepath"
	"testing"
)

// Codex resolves a CODEX_HOME it is given through every link and keys hook
// trust by that real path, while it names its default home/.codex as
// given; the receipt names the hooks file the way Codex keys it.
func TestCodexHooksPathNamesTheHomeAsCodexKeysIt(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name        string
		dotDir, set bool
	}{
		{"configured home through a link", false, true},
		{"default home through a link", true, false},
		{"configured home naming the default through a link", true, true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			options := codexOptions(t, "user")
			target := filepath.Join(options.Home, "codex-target")
			must(t, os.MkdirAll(target, 0o700))
			link := filepath.Join(options.Home, "codex-link")
			if row.dotDir {
				link = filepath.Join(options.Home, ".codex")
				must(t, os.RemoveAll(link))
			}
			if err := os.Symlink(target, link); err != nil {
				t.Skipf("no symbolic link can be made here: %v", err)
			}
			options.ConfigDir, options.ConfigDirSet = link, row.set
			plan, err := PlanCodex(options)
			must(t, err)
			want := filepath.Join(link, "hooks.json")
			if row.set {
				want = filepath.Join(target, "hooks.json")
			}
			if plan.Receipt.HooksPath != want {
				t.Fatalf("hooks path = %q, want %q", plan.Receipt.HooksPath, want)
			}
		})
	}
}

package proof_test

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/proof"
)

func TestLoadManifest(t *testing.T) {
	t.Parallel()
	root := fixtureRoot(t)
	m := manifest(t, root)
	if got := m.Names(); !slices.Equal(got, []string{"shell.no-verify", "text.dashes"}) {
		t.Errorf("Names() = %v", got)
	}
	e, ok := m.Lookup("text.dashes")
	if !ok || e.Code != "em-dash" || e.Red != "testdata/text.dashes/red" || e.Near != "testdata/text.dashes/near" {
		t.Errorf("Lookup = %+v, %v", e, ok)
	}
	if _, ok := m.Lookup("text.todo"); ok {
		t.Error("Lookup must not find an absent id")
	}
}

func TestLoadManifestRefusesBrokenFiles(t *testing.T) {
	t.Parallel()
	bad := map[string]string{
		"unknown key":  "checks:\n  - {id: text.dashes, code: em-dash, red: r, near: n, extra: x}\n",
		"repeated id":  "checks:\n  - {id: text.dashes, code: em-dash, red: r, near: n}\n  - {id: text.dashes, code: em-dash, red: r, near: n}\n",
		"bad id":       "checks:\n  - {id: Dashes, code: em-dash, red: r, near: n}\n",
		"bad code":     "checks:\n  - {id: text.dashes, code: EM_DASH, red: r, near: n}\n",
		"missing near": "checks:\n  - {id: text.dashes, code: em-dash, red: r}\n",
		"not yaml":     "checks: [\n",
	}
	for name, content := range bad {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			write(t, root, "manifest.yml", content)
			if _, err := proof.LoadManifest(filepath.Join(root, "manifest.yml")); !errors.Is(err, proof.ErrManifest) {
				t.Fatalf("LoadManifest() = %v, want ErrManifest", err)
			}
		})
	}
	if _, err := proof.LoadManifest(filepath.Join(t.TempDir(), "absent.yml")); !errors.Is(err, proof.ErrManifest) {
		t.Errorf("absent manifest: %v", err)
	}
}

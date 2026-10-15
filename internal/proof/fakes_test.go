// The fixtures and helpers every proof test shares: a fixture root with the
// red and near-miss trees of two fake checks, and the manifest naming them.
package proof_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/proof"
)

// escaped is how a fixture file spells the em dash without carrying it.
const escaped = "\\" + "u2014"

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

const shellEvent = `{"shape":"shell","agent":"tester","tool":"Bash","shell":{"command":%q,"cwd":"/work","tool":"Bash"}}`

// fixtureRoot writes the fixtures of the two fake checks and a manifest
// naming them, and returns the root.
func fixtureRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "testdata/text.dashes/red/docs/a.md.esc", "title "+escaped+" subtitle\n")
	write(t, root, "testdata/text.dashes/near/docs/a.md", "title - subtitle\n")
	write(t, root, "testdata/shell.no-verify/red/event.json", fmt.Sprintf(shellEvent, "git commit --no-verify -m x"))
	write(t, root, "testdata/shell.no-verify/near/event.json", fmt.Sprintf(shellEvent, "git commit -m x"))
	write(t, root, "manifest.yml", "checks:\n"+
		"  - {id: text.dashes, code: em-dash, red: testdata/text.dashes/red, near: testdata/text.dashes/near}\n"+
		"  - {id: shell.no-verify, code: no-verify, red: testdata/shell.no-verify/red, near: testdata/shell.no-verify/near}\n")
	return root
}

func manifest(t *testing.T, root string) proof.Manifest {
	t.Helper()
	m, err := proof.LoadManifest(filepath.Join(root, "manifest.yml"))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

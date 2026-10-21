package setup

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/install"
)

// errLefthook is the failure setup restores the hooks after.
var errLefthook = errors.New("lefthook failed")

// A hook lefthook rewrote in place gets its bytes back, and a hook whose
// directory is gone names the hook setup could not put back.
func TestRestoreHookRewritesInPlaceAndNamesWhatItCannotRestore(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "pre-commit")
	if err := os.WriteFile(path, []byte("lefthook hook\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cause := errLefthook
	err := restoreHooks([]install.Change{{Path: path, Before: []byte("old lefthook hook\n"), Existed: true}}, cause)
	if !errors.Is(err, cause) || !strings.Contains(err.Error(), "setup put the git hooks back") {
		t.Fatalf("restore = %v", err)
	}
	if data, readErr := os.ReadFile(path); readErr != nil || string(data) != "old lefthook hook\n" {
		t.Fatalf("hook = %q, %v", data, readErr)
	}
	gone := filepath.Join(dir, "gone", "pre-push")
	err = restoreHooks([]install.Change{{Path: gone, Before: []byte("hook\n"), Existed: true}}, cause)
	if !errors.Is(err, cause) || !strings.Contains(err.Error(), "could not put every git hook back") || !strings.Contains(err.Error(), gone) {
		t.Fatalf("restore without the directory = %v", err)
	}
}

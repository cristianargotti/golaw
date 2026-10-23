package profile_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/profile"
)

// twoFilesDigest is the digest of a bundle holding a.txt ("a\n") and
// b.txt ("b\n"): the SHA-256 of "a.txt\n<sha256 of a\n>\nb.txt\n<sha256 of b\n>\n".
const twoFilesDigest = "sha256:a61b93fe6be27cfb02756d02828d57da45b74e12e5775467c80b6c7553e442cd"

func writeBundle(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestDigestCoversEveryFileButTheStamp(t *testing.T) {
	t.Parallel()
	dir := writeBundle(t, map[string]string{"b.txt": "b\n", "a.txt": "a\n", "profile.yml": "name: x\n"})
	got, err := profile.Digest(dir)
	if err != nil || got != twoFilesDigest {
		t.Fatalf("Digest = %s, %v, want %s", got, err, twoFilesDigest)
	}
	withGit := writeBundle(t, map[string]string{"b.txt": "b\n", "a.txt": "a\n", ".git/HEAD": "ref: refs/heads/main\n", "profile.yml": "other\n"})
	if got, err := profile.Digest(withGit); err != nil || got != twoFilesDigest {
		t.Errorf("a .git directory and the stamp are left out: %s, %v", got, err)
	}
	nested := writeBundle(t, map[string]string{"b.txt": "b\n", "a.txt": "a\n", "docs/profile.yml": "x\n"})
	if got, err := profile.Digest(nested); err != nil || got == twoFilesDigest {
		t.Errorf("a nested profile.yml is data: %s, %v", got, err)
	}
	changed := writeBundle(t, map[string]string{"b.txt": "b\n", "a.txt": "a!\n"})
	if got, err := profile.Digest(changed); err != nil || got == twoFilesDigest {
		t.Errorf("one byte changes the digest: %s, %v", got, err)
	}
}

func TestDigestRefusesAnIrregularEntry(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links need a privilege on windows")
	}
	dir := writeBundle(t, map[string]string{"a.txt": "a\n"})
	if err := os.Symlink(filepath.Join(dir, "a.txt"), filepath.Join(dir, "link.txt")); err != nil {
		t.Fatal(err)
	}
	_, err := profile.Digest(dir)
	if !errors.Is(err, profile.ErrBundle) || !errors.Is(err, model.ErrPolicy) {
		t.Errorf("a link in a bundle is ErrBundle under ErrPolicy: %v", err)
	}
}

func TestDigestOfAMissingDirectory(t *testing.T) {
	t.Parallel()
	if _, err := profile.Digest(filepath.Join(t.TempDir(), "absent")); !errors.Is(err, profile.ErrBundle) {
		t.Errorf("an absent directory is ErrBundle: %v", err)
	}
}

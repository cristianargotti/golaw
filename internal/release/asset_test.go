package release_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/release"
)

// helloDigest is the SHA-256 of "hello\n".
const helloDigest = "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03"

func TestAssetNameFollowsTheArchiveTemplate(t *testing.T) {
	t.Parallel()
	for name, want := range map[[3]string]string{
		{"1.2.3", "darwin", "arm64"}:  "golaw_1.2.3_darwin-arm64",
		{"1.2.3", "linux", "amd64"}:   "golaw_1.2.3_linux-amd64",
		{"1.2.3", "windows", "amd64"}: "golaw_1.2.3_windows-amd64.exe",
		{"0.1.0", "windows", "arm64"}: "golaw_0.1.0_windows-arm64.exe",
	} {
		if got := release.AssetName(name[0], name[1], name[2]); got != want {
			t.Errorf("AssetName(%v) = %s, want %s", name, got, want)
		}
	}
}

func TestFileDigestHashesAFileAndNamesAMissingOne(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "hello")
	if err := os.WriteFile(path, []byte("hello\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := release.FileDigest(path); err != nil || got != helloDigest {
		t.Errorf("FileDigest = %s, %v", got, err)
	}
	absent := filepath.Join(t.TempDir(), "absent")
	_, err := release.FileDigest(absent)
	if !errors.Is(err, release.ErrInput) || !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a missing file is ErrInput over the cause: %v", err)
	}
}

func TestIdentitySubjectNamesTheTag(t *testing.T) {
	t.Parallel()
	id := release.Identity{Lane: release.Lane, Issuer: release.DefaultIssuer}
	want := "https://github.com/cristianargotti/golaw/.github/workflows/release.yml@refs/tags/v1.2.3"
	if got := id.Subject("1.2.3"); got != want {
		t.Errorf("Subject = %s, want %s", got, want)
	}
	test := release.Identity{Lane: "https://example.test/.github/workflows/release.yml", Issuer: release.DefaultIssuer}
	if got := test.Subject("0.9.0"); got != "https://example.test/.github/workflows/release.yml@refs/tags/v0.9.0" {
		t.Errorf("a test identity: %s", got)
	}
}

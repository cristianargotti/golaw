package release_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/release"
)

// checksums is a checksums.txt as goreleaser writes it, with one line in
// the binary mode of sha256sum, a blank line and a line without a name.
const checksums = helloDigest + "  golaw_1.2.3_darwin-arm64\n" +
	"ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789 *golaw_1.2.3_linux-amd64\n" +
	"\n" +
	"0000000000000000000000000000000000000000000000000000000000000000\n" +
	helloDigest + "  golaw_1.2.3_windows-amd64.exe\n"

func TestChecksumOfReadsTheAssetLine(t *testing.T) {
	t.Parallel()
	for asset, want := range map[string]string{
		"golaw_1.2.3_darwin-arm64":      helloDigest,
		"golaw_1.2.3_linux-amd64":       strings.ToLower("ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789"),
		"golaw_1.2.3_windows-amd64.exe": helloDigest,
	} {
		if got, ok := release.ChecksumOf([]byte(checksums), asset); !ok || got != want {
			t.Errorf("ChecksumOf(%s) = %s, %v, want %s", asset, got, ok, want)
		}
	}
	for _, asset := range []string{"golaw_1.2.3_linux-arm64", "golaw_1.2.3_darwin-arm64.exe", "", "*golaw_1.2.3_linux-amd64"} {
		if got, ok := release.ChecksumOf([]byte(checksums), asset); ok {
			t.Errorf("ChecksumOf(%q) = %s, want no line", asset, got)
		}
	}
	if _, ok := release.ChecksumOf(nil, "golaw_1.2.3_darwin-arm64"); ok {
		t.Error("an empty file names no asset")
	}
}

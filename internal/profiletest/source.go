package profiletest

import (
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
)

// BareSource is a bare repository under the temporary directory of tb
// holding the fixture bundle committed and tagged Tag. It answers the
// directory path, which git clones as a URL, and the tag; every install
// test is served by it and none reaches the network.
func BareSource(tb testing.TB) (string, string) {
	tb.Helper()
	work := gittest.New(tb)
	work.Commit("feat: the fixture bundle", files(tb))
	work.Git("tag", Tag)
	bare := filepath.Join(filepath.Dir(work.Root), "source.git")
	work.Git("clone", "-q", "--bare", work.Root, bare)
	return bare, Tag
}

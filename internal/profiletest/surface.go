package profiletest

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/profile"
)

// The identity of the surface bundle, a second synthetic bundle that
// carries the two data files of M6.3.3.
const (
	SurfaceName    = "surface"
	SurfaceVersion = "2.0.0"
	SurfaceSource  = "https://example.test/profiles/surface"
	surfaceDate    = "2026-10-27"
)

// SurfaceDir is internal/profiletest/testdata/surface of the module. The
// directory holds no stamp: SurfaceInstall writes it with the digest of
// the files, so a change to a fixture file never leaves a stale digest.
func SurfaceDir(tb testing.TB) string {
	tb.Helper()
	internal := filepath.Dir(filepath.Dir(filepath.Dir(Dir(tb))))
	return filepath.Join(internal, "profiletest", "testdata", SurfaceName)
}

// SurfaceRef is the pinned reference of the surface bundle, its digest
// computed over the fixture files.
func SurfaceRef(tb testing.TB) policy.ProfileRef {
	tb.Helper()
	digest, err := profile.Digest(SurfaceDir(tb))
	must(tb, err)
	return policy.ProfileRef{Name: SurfaceName, Source: SurfaceSource, Version: SurfaceVersion, Digest: digest}
}

// SurfaceInstall copies the surface bundle into the current bundle
// directory of its name under configDir, writes its stamp and answers it.
func SurfaceInstall(tb testing.TB, configDir string) policy.Stamp {
	tb.Helper()
	dst := profile.Dir(configDir, SurfaceName)
	for rel, content := range surfaceFiles(tb) {
		target := filepath.Join(dst, filepath.FromSlash(rel))
		must(tb, os.MkdirAll(filepath.Dir(target), dirMode))
		must(tb, os.WriteFile(target, content, fileMode))
	}
	return SurfaceStamp(tb, configDir, SurfaceRef(tb).Digest)
}

// SurfaceStamp writes the stamp of the installed surface bundle with a
// digest and answers it, so a test that edits a data file of the copy
// pins the edited bytes and reaches the readers behind the digest.
func SurfaceStamp(tb testing.TB, configDir, digest string) policy.Stamp {
	tb.Helper()
	path := filepath.Join(profile.Dir(configDir, SurfaceName), policy.StampFile)
	stamp := fmt.Sprintf("name: %s\nversion: %s\ndate: %s\nsource: %s\ndigest: %s\n", SurfaceName, SurfaceVersion, surfaceDate, SurfaceSource, digest)
	must(tb, os.WriteFile(path, []byte(stamp), fileMode))
	s, err := policy.LoadStamp(path)
	must(tb, err)
	return s
}

// surfaceFiles reads the surface bundle, which holds one directory, as the
// slash path of each file to its content.
func surfaceFiles(tb testing.TB) map[string][]byte {
	tb.Helper()
	tree := os.DirFS(SurfaceDir(tb))
	out := map[string][]byte{}
	must(tb, fs.WalkDir(tree, ".", func(rel string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, readErr := fs.ReadFile(tree, rel)
		must(tb, readErr)
		out[rel] = data
		return nil
	}))
	return out
}

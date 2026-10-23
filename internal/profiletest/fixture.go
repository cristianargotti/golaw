// Package profiletest serves the synthetic fixture bundle of the kit to
// tests (D16): where it lives, its reference and data, a copy installed
// under a temporary config directory, a tampered copy, and a local bare
// repository that serves it at its tag so no test reaches the network.
package profiletest

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/profile"
)

// The identity of the fixture bundle, as its stamp carries it.
const (
	Name    = "fixture"
	Version = "1.2.3"
	Source  = "https://example.test/profiles/fixture"
	Tag     = "v1.2.3"
)

// The modes of what the helpers write under a temporary directory.
const (
	dirMode  = 0o750
	fileMode = 0o600
)

// Dir is internal/profile/testdata/fixture of the module, found by walking
// up from the working directory of the test to go.mod.
func Dir(tb testing.TB) string {
	tb.Helper()
	wd, err := os.Getwd()
	must(tb, err)
	for dir := wd; ; dir = filepath.Dir(dir) {
		_, err := os.Stat(filepath.Join(dir, "go.mod"))
		switch {
		case err == nil:
			return filepath.Join(dir, "internal", "profile", "testdata", "fixture")
		case !errors.Is(err, fs.ErrNotExist):
			tb.Fatal(err)
		case filepath.Dir(dir) == dir:
			tb.Fatalf("no go.mod above %s", wd)
		}
	}
}

// Ref is the pinned reference of the fixture: its name, source, version
// and the digest the stamp carries.
func Ref(tb testing.TB) policy.ProfileRef {
	tb.Helper()
	s := stamp(tb)
	return policy.ProfileRef{Name: s.Name, Source: s.Source, Version: s.Version, Digest: s.Digest}
}

// Data is the fixture bundle read and verified, Name, Version and Digest
// filled.
func Data(tb testing.TB) *model.ProfileData {
	tb.Helper()
	b, err := profile.Read(Dir(tb))
	must(tb, err)
	return &b.Data
}

// Install copies the bundle into the current bundle directory of its name
// under configDir and answers its stamp.
func Install(tb testing.TB, configDir string) policy.Stamp {
	tb.Helper()
	dst := profile.Dir(configDir, Name)
	for rel, content := range files(tb) {
		target := filepath.Join(dst, filepath.FromSlash(rel))
		must(tb, os.MkdirAll(filepath.Dir(target), dirMode))
		must(tb, os.WriteFile(target, []byte(content), fileMode))
	}
	return stamp(tb)
}

// Tampered is Install with one byte appended to rules.yml: the stamp still
// names the original digest, the files no longer hash to it (D8).
func Tampered(tb testing.TB, configDir string) policy.Stamp {
	tb.Helper()
	s := Install(tb, configDir)
	path := filepath.Join(profile.Dir(configDir, Name), profile.RulesFile)
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, fileMode) //nolint:gosec // the path is under the temporary config directory
	must(tb, err)
	_, err = file.WriteString("\n")
	must(tb, err)
	must(tb, file.Close())
	return s
}

// stamp reads the stamp of the fixture.
func stamp(tb testing.TB) policy.Stamp {
	tb.Helper()
	s, err := policy.LoadStamp(filepath.Join(Dir(tb), policy.StampFile))
	must(tb, err)
	return s
}

// files reads the fixture bundle, which is flat by design, as the map a
// commit takes: the slash path of each file to its content.
func files(tb testing.TB) map[string]string {
	tb.Helper()
	root := Dir(tb)
	entries, err := os.ReadDir(root)
	must(tb, err)
	out := make(map[string]string, len(entries))
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(root, entry.Name())) //nolint:gosec // the path is an entry of the fixture directory
		must(tb, err)
		out[entry.Name()] = string(data)
	}
	return out
}

// must fails the test on an error of a helper.
func must(tb testing.TB, err error) {
	tb.Helper()
	if err != nil {
		tb.Fatal(err)
	}
}

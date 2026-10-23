package profile_test

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/profile"
	"github.com/cristianargotti/golaw/internal/profiletest"
)

// release is one tagged version a test source serves: the fixture files
// with some replaced, so a version carries a digest of its own.
type release struct {
	name    string            // the name the stamp carries; the fixture's when empty
	version string            // the version the stamp carries
	tag     string            // the tag; "v" and the version when empty
	files   map[string]string // files replacing the fixture's, by slash path
	digest  string            // the stamp digest; the one the files hash to when empty
}

// fixtureFiles reads the flat fixture bundle as slash path to content.
func fixtureFiles(t *testing.T) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(profiletest.Dir(t))
	if err != nil {
		t.Fatal(err)
	}
	out := make(map[string]string, len(entries))
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(profiletest.Dir(t), entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		out[entry.Name()] = string(data)
	}
	return out
}

// bundleSource is a local bare repository serving each release committed
// in order and tagged, the way an author publishes versions; it answers
// the URL git clones. No test reaches the network.
func bundleSource(t *testing.T, releases ...release) string {
	t.Helper()
	work := gittest.New(t)
	for _, r := range releases {
		files := fixtureFiles(t)
		delete(files, policy.StampFile)
		maps.Copy(files, r.files)
		for rel, content := range files {
			work.Write(rel, content)
		}
		digest := r.digest
		if digest == "" {
			computed, err := profile.Digest(work.Root)
			if err != nil {
				t.Fatal(err)
			}
			digest = computed
		}
		name := r.name
		if name == "" {
			name = profiletest.Name
		}
		files[policy.StampFile] = fmt.Sprintf("name: %s\nversion: %s\ndate: 2026-10-20\nsource: %s\ndigest: %s\n",
			name, r.version, profiletest.Source, digest)
		work.Commit("feat: the fixture bundle at "+r.version, files)
		tag := r.tag
		if tag == "" {
			tag = "v" + r.version
		}
		work.Git("tag", tag)
	}
	bare := filepath.Join(t.TempDir(), "source.git")
	work.Git("clone", "-q", "--bare", work.Root, bare)
	return bare
}

// note is a release whose onboarding page differs, and so its digest.
func note(version, text string) release {
	return release{version: version, files: map[string]string{"ONBOARDING.md": text + "\n"}}
}

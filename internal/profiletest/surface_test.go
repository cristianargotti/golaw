package profiletest_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/profile"
	"github.com/cristianargotti/golaw/internal/profiletest"
)

func TestTheSurfaceBundleCarriesBothDataFiles(t *testing.T) {
	t.Parallel()
	dir := profiletest.SurfaceDir(t)
	for _, name := range []string{profile.RulesFile, profile.AnalyzerFile, filepath.Join(profile.SonarWayDir, "js.yml")} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("the surface bundle holds %s: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, policy.StampFile)); err == nil {
		t.Error("the surface bundle commits no stamp: SurfaceInstall writes it")
	}
}

func TestSurfaceInstallWritesAStampThatVerifies(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	stamp := profiletest.SurfaceInstall(t, config)
	ref := profiletest.SurfaceRef(t)
	if stamp.Name != profiletest.SurfaceName || stamp.Version != profiletest.SurfaceVersion || stamp.Digest != ref.Digest {
		t.Fatalf("stamp = %+v, ref = %+v", stamp, ref)
	}
	if _, err := policy.VerifyProfile(config, ref); err != nil {
		t.Fatalf("the installed bundle verifies against the reference: %v", err)
	}
	if got, err := profile.Digest(profile.Dir(config, profiletest.SurfaceName)); err != nil || got != ref.Digest {
		t.Fatalf("the copy hashes to the reference: %s, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(profile.Dir(config, profiletest.SurfaceName), profile.SonarWayDir, "js.yml")); err != nil {
		t.Fatalf("the copy keeps the snapshot directory: %v", err)
	}
}

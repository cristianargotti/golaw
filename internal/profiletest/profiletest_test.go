package profiletest_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/profile"
	"github.com/cristianargotti/golaw/internal/profiletest"
)

func TestFixtureIsFoundFromAnyPackage(t *testing.T) {
	t.Parallel()
	dir := profiletest.Dir(t)
	for _, name := range []string{policy.StampFile, profile.RulesFile, "ONBOARDING.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("the fixture holds %s: %v", name, err)
		}
	}
	ref, data := profiletest.Ref(t), profiletest.Data(t)
	if ref.Name != profiletest.Name || ref.Version != profiletest.Version || ref.Source != profiletest.Source {
		t.Errorf("Ref = %+v", ref)
	}
	if data.Name != ref.Name || data.Digest != ref.Digest || len(data.Terms) == 0 {
		t.Errorf("Data = %+v", data)
	}
}

func TestInstallWritesTheCurrentBundle(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	stamp := profiletest.Install(t, config)
	if stamp.Digest != profiletest.Ref(t).Digest {
		t.Errorf("Install answers the stamp: %+v", stamp)
	}
	if _, err := policy.VerifyProfile(config, profiletest.Ref(t)); err != nil {
		t.Errorf("the installed bundle verifies against the reference: %v", err)
	}
	if got, err := profile.Digest(profile.Dir(config, profiletest.Name)); err != nil || got != stamp.Digest {
		t.Errorf("the copy hashes to the stamp: %s, %v", got, err)
	}
	tampered := t.TempDir()
	if s := profiletest.Tampered(t, tampered); s.Digest != stamp.Digest {
		t.Errorf("Tampered keeps the stamp and changes the files: %+v", s)
	}
	if got, err := profile.Digest(profile.Dir(tampered, profiletest.Name)); err != nil || got == stamp.Digest {
		t.Errorf("the tampered copy hashes to another digest: %s, %v", got, err)
	}
}

func TestBareSourceServesTheFixtureAtItsTag(t *testing.T) {
	t.Parallel()
	url, tag := profiletest.BareSource(t)
	if tag != profiletest.Tag {
		t.Errorf("tag = %s", tag)
	}
	clone := filepath.Join(t.TempDir(), "clone")
	cmd := exec.CommandContext(t.Context(), "git", "clone", "-q", "--depth", "1", "--branch", tag, url, clone)
	cmd.Env = append(os.Environ(), gittest.Isolated()...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git clone: %v: %s", err, out)
	}
	b, err := profile.Read(clone)
	if err != nil || b.Stamp.Digest != profiletest.Ref(t).Digest {
		t.Errorf("the clone reads as the fixture: %+v, %v", b, err)
	}
}

package lint

import (
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
	"github.com/cristianargotti/golaw/internal/surfacefix"
)

func TestStrictDockerAssetsHoldTheirProof(t *testing.T) {
	t.Parallel()
	proveInfraSet(t, strictDockerManifest, model.SurfaceStrict, infraDrivers())
}

// TestStrictDockerEntriesStayOutOfParity: an info rule of hadolint is
// ignored by the parity file, and checkov reads no Dockerfile under parity.
func TestStrictDockerEntriesStayOutOfParity(t *testing.T) {
	t.Parallel()
	m, err := proof.LoadManifest(strictDockerManifest)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range m.Checks {
		fixture, err := proof.LoadFixture(filepath.Join(filepath.Dir(strictDockerManifest), e.Red))
		if err != nil {
			t.Fatal(err)
		}
		issues, ran := infraRun(t, infraEntryDriver(t, infraDrivers(), e), model.SurfaceParity, fixture.Files)
		if ran && infraCodes(issues)[e.Code] > 0 {
			t.Errorf("%s fires under parity: %+v", e.ID, issues)
		}
	}
}

// TestARepositoryHadolintConfigChangesNoResult is M6.10 row (7): a
// .hadolint.yaml in the directory hadolint starts in ignores DL3002, and the
// kit's configuration still reports it.
func TestARepositoryHadolintConfigChangesNoResult(t *testing.T) {
	t.Parallel()
	fixture, err := proof.LoadFixture("testdata/infra/dl3002/red")
	if err != nil {
		t.Fatal(err)
	}
	fixture.Files[".hadolint.yaml"] = []byte("ignored:\n  - DL3002\n")
	for _, set := range infraSets {
		issues, ran := infraRun(t, hadolintDriver(), set, fixture.Files)
		if !ran || infraCodes(issues)["dl3002"] != 1 {
			t.Errorf("%s: the repository file hid DL3002: %+v", set, issues)
		}
	}
}

// TestHadolintLeavesTheDigestShapesToTheNativeCheck pins two facts measured
// with hadolint 2.15.1: a FROM with a tag and a digest raises nothing
// (survey 2.2), so docker:S8431 rests on docker.tag-and-digest alone, and a
// digest-only FROM raises no DL3006 (risk 12), so the fix never trades one
// finding for another.
func TestHadolintLeavesTheDigestShapesToTheNativeCheck(t *testing.T) {
	t.Parallel()
	for name, twin := range map[string]surfacefix.Twin{"tag and digest": surfacefix.Red, "digest only": surfacefix.Corrected} {
		dockerfile := map[string][]byte{"Dockerfile": infraTwin(t, twin)["Dockerfile"]}
		for _, set := range infraSets {
			issues, ran := infraRun(t, hadolintDriver(), set, dockerfile)
			if !ran || len(issues) != 0 {
				t.Errorf("%s under %s: %+v", name, set, issues)
			}
		}
	}
}

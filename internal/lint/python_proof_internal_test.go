package lint

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

func pythonManifestPath() string { return filepath.Join("proof", "python.yml") }

func pythonProofID(r Row) string { return "lint." + r.Tool + "-" + Code(r.Rule) }

// pythonProofDrivers leaves out the format check: it maps no Sonar key and
// shares its tool name with ruff check.
func pythonProofDrivers() []Driver {
	return slices.DeleteFunc(pythonDrivers(), func(d Driver) bool { return d.Format == FormatRuffFormat })
}

func TestPythonEveryRowHasItsProofEntry(t *testing.T) {
	t.Parallel()
	m, err := proof.LoadManifest(pythonManifestPath())
	if err != nil {
		t.Fatalf("load the manifest: %v", err)
	}
	want := make(map[string]string)
	for _, r := range pythonRows() {
		want[pythonProofID(r)] = Code(r.Rule)
	}
	got := make(map[string]string, len(m.Checks))
	for _, e := range m.Checks {
		got[e.ID] = e.Code
		for _, dir := range []string{e.Red, e.Near} {
			if info, statErr := os.Stat(filepath.Join("proof", filepath.FromSlash(dir))); statErr != nil || !info.IsDir() {
				t.Errorf("%s: fixture %s is not a directory: %v", e.ID, dir, statErr)
			}
		}
	}
	if !maps.Equal(got, want) {
		t.Errorf("manifest entries = %v, the rows want %v", got, want)
	}
}

func TestPythonRowsAreProvenByTheirFixtures(t *testing.T) {
	t.Parallel()
	proveManifest(t, pythonManifestPath(), pythonProofDrivers())
}

func TestPythonFormatCheckRedAndNear(t *testing.T) {
	t.Parallel()
	d := pythonDriver(t, FormatRuffFormat)
	redRoot, redFiles := pythonFixtureTree(t, filepath.Join("testdata", "python", "format", "red"))
	red := runPythonDriver(t, d, model.SurfaceStrict, redRoot, redFiles)
	if len(red) == 0 {
		t.Error("the unformatted file raised no format finding")
	}
	if got := pythonCodes(red)["format"]; got > 1 {
		t.Errorf("the one unformatted file raised %d format findings, want one per file", got)
	}
	for _, i := range red {
		if i.Path != "deploy/report.py" {
			t.Errorf("format finding on %q, want deploy/report.py", i.Path)
		}
	}
	nearRoot, nearFiles := pythonFixtureTree(t, filepath.Join("testdata", "python", "format", "near"))
	if near := runPythonDriver(t, d, model.SurfaceStrict, nearRoot, nearFiles); len(near) != 0 {
		t.Errorf("the formatted file raised %v", near)
	}
}

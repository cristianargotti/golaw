package lint

import (
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

func pythonConfigFixture(name string) string {
	return filepath.Join("testdata", "python", "config", name)
}

// pythonSilences answers whether an ignore of the set, global or per file,
// covers the code, a prefix such as S or S1 included.
func pythonSilences(cfg pythonRuffLint, code string) bool {
	covers := func(entries []string) bool {
		return slices.ContainsFunc(entries, func(entry string) bool { return strings.HasPrefix(code, entry) })
	}
	if covers(cfg.Ignore) {
		return true
	}
	for _, entries := range cfg.PerFileIgnores {
		if covers(entries) {
			return true
		}
	}
	return false
}

func TestPythonNoSetSilencesACredentialRule(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		set      string
		code     string
		silenced bool
	}{
		"parity keeps S105":                                     {set: model.SurfaceParity, code: "S105"},
		"parity keeps S106":                                     {set: model.SurfaceParity, code: "S106"},
		"parity keeps S107":                                     {set: model.SurfaceParity, code: "S107"},
		"strict keeps S105, tests included":                     {set: model.SurfaceStrict, code: "S105"},
		"strict keeps S106, tests included":                     {set: model.SurfaceStrict, code: "S106"},
		"strict keeps S107, tests included":                     {set: model.SurfaceStrict, code: "S107"},
		"strict allows asserts in tests (near miss: the allow)": {set: model.SurfaceStrict, code: "S101", silenced: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := pythonSilences(pythonReadRuff(t, tc.set), tc.code); got != tc.silenced {
				t.Errorf("%s silences %s = %v, want %v", tc.set, tc.code, got, tc.silenced)
			}
		})
	}
}

//nolint:paralleltest // the live runs resolve the tools through the test environment
func TestPythonTestFilesKeepAssertsButNeverCredentials(t *testing.T) {
	tests := map[string]struct {
		tree, present, absent string
	}{
		"a test file keeps its asserts, never its literal credential":                {tree: "tests", present: "s105", absent: "s101"},
		"a main file loses its asserts (near miss: the allow follows the test name)": {tree: "main", present: "s101"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			root, files := pythonFixtureTree(t, pythonConfigFixture(tc.tree))
			codes := pythonCodes(runPythonDriver(t, pythonDriver(t, FormatRuff), model.SurfaceStrict, root, files))
			if codes[tc.present] == 0 {
				t.Errorf("strict raised %v, want %s", codes, tc.present)
			}
			if tc.absent != "" && codes[tc.absent] != 0 {
				t.Errorf("strict raised %s on a test file: %v", tc.absent, codes)
			}
		})
	}
}

//nolint:paralleltest // the live runs resolve the tools through the test environment
func TestPythonRepositoryConfigChangesNoResult(t *testing.T) {
	for _, set := range []string{model.SurfaceParity, model.SurfaceStrict} {
		t.Run(set, func(t *testing.T) {
			root, files := pythonFixtureTree(t, pythonConfigFixture("repo"))
			ruff := pythonCodes(runPythonDriver(t, pythonDriver(t, FormatRuff), set, root, files))
			for _, code := range []string{"s105", "s307"} {
				if ruff[code] == 0 {
					t.Errorf("%s: the repository config silenced %s: %v", set, code, ruff)
				}
			}
			pyright := pythonCodes(runPythonDriver(t, pythonDriver(t, FormatPyright), set, root, files))
			if pyright["reportassignmenttype"] == 0 {
				t.Errorf("%s: the repository config silenced reportAssignmentType: %v", set, pyright)
			}
		})
	}
}

//nolint:paralleltest // the live runs resolve the tools through the test environment
func TestPythonStrictPyrightJudgesTheFileNotItsImports(t *testing.T) {
	tests := map[string]struct {
		tree string
		want string
	}{
		"an import of the repository's own module raises nothing": {tree: "imports"},
		"an unannotated parameter is still judged (near miss)":    {tree: "untyped", want: "reportmissingparametertype"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			root, files := pythonFixtureTree(t, pythonConfigFixture(tc.tree))
			codes := pythonCodes(runPythonDriver(t, pythonDriver(t, FormatPyright), model.SurfaceStrict, root, files))
			if tc.want == "" && len(codes) != 0 {
				t.Errorf("strict pyright raised %v on correct imports", codes)
			}
			if tc.want != "" && codes[tc.want] == 0 {
				t.Errorf("strict pyright raised %v, want %s", codes, tc.want)
			}
		})
	}
}

//nolint:paralleltest // the live runs resolve the tools through the test environment
func TestPythonLintWritesNothingIntoTheCheckout(t *testing.T) {
	root, files := pythonFixtureTree(t, pythonConfigFixture("repo"))
	before := pythonListing(t, root)
	for _, format := range []Format{FormatRuff, FormatRuffFormat, FormatPyright} {
		runPythonDriver(t, pythonDriver(t, format), model.SurfaceStrict, root, files)
	}
	if after := pythonListing(t, root); !slices.Equal(before, after) {
		t.Errorf("the lint run changed the checkout: before %v, after %v", before, after)
	}
}

func pythonListing(t *testing.T, root string) []string {
	t.Helper()
	var paths []string
	err := filepath.WalkDir(root, func(path string, _ fs.DirEntry, walkErr error) error {
		paths = append(paths, filepath.ToSlash(path))
		return walkErr
	})
	if err != nil {
		t.Fatalf("list %s: %v", root, err)
	}
	return paths
}

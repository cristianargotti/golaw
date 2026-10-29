package profile

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

// bundleWith writes files under a temporary bundle directory.
func bundleWith(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestReadAnalyzerReadsTheScopeOfAPipeline(t *testing.T) {
	t.Parallel()
	dir := bundleWith(t, map[string]string{AnalyzerFile: `sources: [cmd, internal]
tests: [test]
test_inclusions: ["**/*_test.go"]
exclusions: ["**/gen/**", "integration/**"]
coverage_exclusions: [examples/*]
coverage_report: build/profile.cov
coverage_profile: {coverpkg: true, covermode: atomic}
`})
	got, found, err := readAnalyzer(dir, "surface")
	if err != nil || !found {
		t.Fatalf("readAnalyzer = %v, %v", found, err)
	}
	if !slices.Equal(got.Sources, []string{"cmd", "internal"}) || !slices.Equal(got.Exclusions, []string{"**/gen/**", "integration/**"}) ||
		got.CoverageReport != "build/profile.cov" || got.Coverage != (model.CoverProfile{CoverPkg: true, CoverMode: "atomic"}) {
		t.Fatalf("analyzer = %+v", got)
	}
	minimal, found, err := readAnalyzer(bundleWith(t, map[string]string{AnalyzerFile: "coverage_profile: {coverpkg: false}\n"}), "surface")
	if err != nil || !found || minimal.Coverage.CoverMode != "set" || minimal.CoverageReport != "" {
		t.Fatalf("a minimal file = %+v, %v, %v, want mode set and no report", minimal, found, err)
	}
	if _, found, err := readAnalyzer(t.TempDir(), "surface"); found || err != nil {
		t.Fatalf("a bundle without analyzer.yml = %v, %v", found, err)
	}
}

func TestReadAnalyzerRefusesWhatTheFileMayNotHold(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		content, cause string
	}{
		"a from key":               {content: "from: auto\n", cause: "field from not found"},
		"an unknown key":           {content: "excludes: [a]\n", cause: "field excludes not found"},
		"an empty file":            {content: "", cause: "the file is empty"},
		"a second document":        {content: "exclusions: [a]\n---\nexclusions: [b]\n", cause: "a second document is not allowed"},
		"an absolute path":         {content: "exclusions: [/srv/**]\n", cause: `exclusions: "/srv/**" is not a clean relative path`},
		"a parent path":            {content: "sources: [../lib]\n", cause: `sources: "../lib" is not a clean relative path`},
		"a path that is not clean": {content: "tests: [a/./b]\n", cause: `tests: "a/./b" is not a clean relative path`},
		"a backslash":              {content: "test_inclusions: ['a\\b']\n", cause: `test_inclusions: "a\\b" is not a clean relative path`},
		"the whole tree as a dot":  {content: "coverage_exclusions: [.]\n", cause: `coverage_exclusions: "." is not a clean relative path`},
		"an entry named twice":     {content: "exclusions: [a/**, a/**]\n", cause: `exclusions: "a/**" is named twice`},
		"a report outside":         {content: "coverage_report: ../profile.cov\n", cause: `coverage_report: "../profile.cov" is not a clean relative path`},
		"a mode go test lacks":     {content: "coverage_profile: {covermode: fast}\n", cause: `coverage_profile.covermode: "fast" is not set, count or atomic`},
		"a list of another type":   {content: "exclusions: a\n", cause: "cannot unmarshal"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, found, err := readAnalyzer(bundleWith(t, map[string]string{AnalyzerFile: tc.content}), "surface")
			if found || !errors.Is(err, ErrBundle) || !errors.Is(err, model.ErrPolicy) ||
				!strings.Contains(err.Error(), "profile surface: analyzer.yml: ") || !strings.Contains(err.Error(), tc.cause) {
				t.Fatalf("readAnalyzer = %v, %v, want ErrBundle naming %q", found, err, tc.cause)
			}
		})
	}
}

func TestReadAnalyzerNamesAFileItCannotRead(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, AnalyzerFile), 0o750); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readAnalyzer(dir, "surface"); !errors.Is(err, ErrBundle) || !strings.Contains(err.Error(), "read analyzer.yml") {
		t.Fatalf("readAnalyzer of a directory = %v", err)
	}
}

package profile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

func TestReadSonarWayJoinsTheLanguageFiles(t *testing.T) {
	t.Parallel()
	dir := bundleWith(t, map[string]string{
		"sonar-way/js.yml": "read: \"2026-10-27\"\nrules:\n  - {key: S2681, severity: MAJOR}\n  - {key: S125, severity: MAJOR}\n",
		"sonar-way/py.yml": "read: \"2026-10-20\"\nrules:\n  - {key: S2068, severity: BLOCKER, params: {credentialWords: \"password,passwd\"}}\n",
		"sonar-way/ts.yml": "read: \"2026-10-27\"\nrules: []\n",
	})
	got, found, err := readSonarWay(dir, "surface")
	if err != nil || !found {
		t.Fatalf("readSonarWay = %v, %v", found, err)
	}
	if got.Read != "2026-10-20" || len(got.Rules) != 3 {
		t.Fatalf("snapshot = %+v, want the oldest date and three rules", got)
	}
	order := []string{"js S125", "js S2681", "py S2068"}
	for i, rule := range got.Rules {
		if rule.Language+" "+rule.Key != order[i] {
			t.Errorf("rule %d = %s %s, want %s", i, rule.Language, rule.Key, order[i])
		}
	}
	if got.Rules[2].Params["credentialWords"] != "password,passwd" || got.Rules[2].Severity != "BLOCKER" {
		t.Errorf("the python rule = %+v", got.Rules[2])
	}
}

func TestReadSonarWayOfNoSnapshotAnswersFalse(t *testing.T) {
	t.Parallel()
	empty := t.TempDir()
	if err := os.Mkdir(filepath.Join(empty, SonarWayDir), 0o750); err != nil {
		t.Fatal(err)
	}
	for name, dir := range map[string]string{"no directory": t.TempDir(), "an empty directory": empty} {
		if _, found, err := readSonarWay(dir, "surface"); found || err != nil {
			t.Errorf("%s = %v, %v, want no snapshot", name, found, err)
		}
	}
}

func TestReadSurfaceStopsAtTheFirstInvalidFile(t *testing.T) {
	t.Parallel()
	for name, files := range map[string]map[string]string{
		"an invalid analyzer.yml": {AnalyzerFile: "from: auto\n"},
		"an invalid snapshot":     {AnalyzerFile: "exclusions: [a/**]\n", "sonar-way/js.yml": "read: never\n"},
	} {
		d := model.ProfileData{Name: "surface"}
		if err := readSurface(bundleWith(t, files), &d); !errors.Is(err, ErrBundle) || d.SonarWay != nil {
			t.Errorf("%s: readSurface = %v, data = %+v", name, err, d)
		}
	}
}

func TestReadSonarWayRefusesWhatTheSnapshotMayNotHold(t *testing.T) {
	t.Parallel()
	languages := "name a snapshot file <language>.yml with one of cloudformation, css, docker, js, py, ts, web"
	tests := map[string]struct {
		files map[string]string
		cause string
	}{
		"a language Sonar way lacks": {files: map[string]string{"sonar-way/go.yml": "read: \"2026-10-27\"\n"}, cause: "sonar-way/go.yml: " + languages},
		"another extension":          {files: map[string]string{"sonar-way/js.yaml": "read: \"2026-10-27\"\n"}, cause: "sonar-way/js.yaml: " + languages},
		"a subdirectory":             {files: map[string]string{"sonar-way/js.yml/a": "x"}, cause: "sonar-way/js.yml: " + languages},
		"a date of another shape":    {files: map[string]string{"sonar-way/js.yml": "read: 27-10-2026\n"}, cause: `read: "27-10-2026" is not a date like 2026-10-27`},
		"no date":                    {files: map[string]string{"sonar-way/js.yml": "rules: []\n"}, cause: `read: "" is not a date`},
		"a key without its S":        {files: map[string]string{"sonar-way/js.yml": "read: \"2026-10-27\"\nrules: [{key: \"2681\", severity: MAJOR}]\n"}, cause: `rules: key "2681" is not a Sonar rule key like S2681`},
		"a key named twice": {
			files: map[string]string{"sonar-way/js.yml": "read: \"2026-10-27\"\nrules: [{key: S125, severity: MAJOR}, {key: S125, severity: MINOR}]\n"},
			cause: "rules: key S125 is named twice",
		},
		"no severity":    {files: map[string]string{"sonar-way/js.yml": "read: \"2026-10-27\"\nrules: [{key: S125}]\n"}, cause: "rules: key S125 has no severity"},
		"an unknown key": {files: map[string]string{"sonar-way/js.yml": "read: \"2026-10-27\"\nprofile: Sonar way\n"}, cause: "field profile not found"},
		"an empty file":  {files: map[string]string{"sonar-way/js.yml": ""}, cause: "sonar-way/js.yml: the file is empty"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, found, err := readSonarWay(bundleWith(t, tc.files), "surface")
			if found || !errors.Is(err, ErrBundle) || !strings.Contains(err.Error(), "profile surface: ") || !strings.Contains(err.Error(), tc.cause) {
				t.Fatalf("readSonarWay = %v, %v, want ErrBundle naming %q", found, err, tc.cause)
			}
		})
	}
}

func TestReadSonarWayNamesWhatItCannotRead(t *testing.T) {
	t.Parallel()
	dir := bundleWith(t, map[string]string{SonarWayDir: "a file where the directory belongs"})
	if _, _, err := readSonarWay(dir, "surface"); !errors.Is(err, ErrBundle) || !strings.Contains(err.Error(), "read sonar-way") {
		t.Fatalf("readSonarWay of a file = %v", err)
	}
}

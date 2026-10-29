package lint

import (
	"errors"
	"io"
	"io/fs"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

func webWay(rules ...model.SonarWayRule) *model.SonarWay {
	return &model.SonarWay{Read: "2026-10-27", Rules: rules}
}

// webProfileWay is the shape of the surface fixture bundle's sonar-way/js.yml
// (contract 2.3): S2681 and S125 active.
func webProfileWay() *model.SonarWay {
	return webWay(model.SonarWayRule{Language: "js", Key: "S2681"}, model.SonarWayRule{Language: "js", Key: "S125"})
}

func webHas(rows []Row, sonar, rule string) bool {
	return slices.ContainsFunc(rows, func(r Row) bool { return r.Sonar == sonar && r.Rule == rule })
}

// TestRunTakesTheWebDriversOfItsPolicy holds the dispatch to the overlay:
// the drivers golaw lint runs for a policy whose profile pins a snapshot
// carry the overlaid rules, and a policy without one runs the public set.
func TestRunTakesTheWebDriversOfItsPolicy(t *testing.T) {
	t.Parallel()
	if got, want := len(driversFor(nil)), len(Drivers()); got != want {
		t.Fatalf("driversFor(nil) = %d drivers, want the %d of Drivers", got, want)
	}
	public := webRulesOf(t, Drivers())
	policy := &model.EffectivePolicy{ProfileData: &model.ProfileData{SonarWay: webProfileWay()}}
	if overlaid := webRulesOf(t, driversFor(policy)); overlaid == public {
		t.Fatal("the eslint driver of a policy with a snapshot carries the public rules, want the overlay")
	}
}

// webRulesOf answers the parity rules the eslint driver of the list carries.
func webRulesOf(t *testing.T, drivers []Driver) string {
	t.Helper()
	for _, d := range drivers {
		if d.Tool != eslintTool {
			continue
		}
		data, err := fs.ReadFile(d.Config, "parity/"+webRulesFile)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	t.Fatal("no eslint driver")
	return ""
}

func TestWebOverlayFollowsTheSnapshot(t *testing.T) {
	t.Parallel()
	idx, err := readSonarIndex()
	if err != nil {
		t.Fatal(err)
	}
	js := webOverlay(webRows(), webProfileWay(), idx)
	css := webOverlay(webRows(), webWay(model.SonarWayRule{Language: "css", Key: "S4647"}), idx)
	cases := []struct {
		name        string
		rows        []Row
		sonar, rule string
		want        bool
	}{
		{"an active public row stays", js, "javascript:S2681", "sonarjs/no-unenclosed-multiline-block", true},
		{"a key the snapshot activates joins", js, "javascript:S125", "sonarjs/no-commented-code", true},
		{"a public row of a covered language the snapshot leaves out drops", js, "javascript:S3358", "sonarjs/no-nested-conditional", false},
		{"a row of a language the snapshot does not cover stays", js, "typescript:S3735", "sonarjs/void-use", true},
		{"a css row stays under a js snapshot", js, "css:S4666", "no-duplicate-selectors", true},
		{"an active css row stays", css, "css:S4647", "color-no-invalid-hex", true},
		{"a css row the css snapshot leaves out drops", css, "css:S4666", "no-duplicate-selectors", false},
	}
	for _, c := range cases {
		if got := webHas(c.rows, c.sonar, c.rule); got != c.want {
			t.Errorf("%s: %s %s present %v, want %v", c.name, c.sonar, c.rule, got, c.want)
		}
	}
}

func TestWebOverlaySkipsUnpinnedPlugins(t *testing.T) {
	t.Parallel()
	idx := sonarIndex{Rules: map[string][]string{"S3863": {"import/no-duplicates"}, "S106": {"no-console"}, "S103": {"@stylistic/max-len"}}}
	way := webWay(
		model.SonarWayRule{Language: "js", Key: "S3863"},
		model.SonarWayRule{Language: "ts", Key: "S106"},
		model.SonarWayRule{Language: "js", Key: "S103"},
	)
	got := webOverlay(nil, way, idx)
	if len(got) != 1 || got[0].Sonar != "typescript:S106" || got[0].Rule != "no-console" || got[0].Set != model.SurfaceParity {
		t.Errorf("webOverlay = %+v, want the one core rule of a pinned plugin", got)
	}
}

func TestWebOverlayWithoutSnapshotIsThePublicMap(t *testing.T) {
	t.Parallel()
	if got := webOverlay(webRows(), nil, sonarIndex{}); !slices.EqualFunc(got, webRows(), func(a, b Row) bool { return a.Sonar == b.Sonar && a.Rule == b.Rule }) {
		t.Error("an overlay without a snapshot changes the public map")
	}
}

func TestWebDriversForAProfile(t *testing.T) {
	t.Parallel()
	plain := webDriversFor(nil)
	overlaid := webDriversFor(&model.EffectivePolicy{ProfileData: &model.ProfileData{SonarWay: webProfileWay()}})
	if len(plain) != 2 || len(overlaid) != 2 {
		t.Fatalf("drivers %d and %d, want two each", len(plain), len(overlaid))
	}
	for _, set := range webSets {
		rules, err := fs.ReadFile(overlaid[0].Config, set+"/"+webRulesFile)
		if err != nil || !strings.Contains(string(rules), `"sonarjs/no-commented-code": "error"`) {
			t.Errorf("the overlaid %s rules miss the snapshot's S125: %v", set, err)
		}
		if strings.Contains(string(rules), "sonarjs/no-nested-conditional") {
			t.Errorf("the overlaid %s rules keep a row the snapshot leaves out", set)
		}
		frame, err := fs.ReadFile(overlaid[1].Config, set+"/"+eslintConfigFile)
		if err != nil || string(frame) != webFile(t, set, eslintConfigFile) {
			t.Errorf("the overlay changed %s/%s: %v", set, eslintConfigFile, err)
		}
		public, err := fs.ReadFile(plain[0].Config, set+"/"+webRulesFile)
		if err != nil || string(public) != webFile(t, set, webRulesFile) {
			t.Errorf("drivers without a snapshot carry other %s rules: %v", set, err)
		}
	}
}

func TestWebReplacedFS(t *testing.T) {
	t.Parallel()
	tree := replacedFS{base: webConfig(), files: map[string][]byte{"parity/" + webRulesFile: []byte("{}\n")}}
	file, err := tree.Open("parity/" + webRulesFile)
	if err != nil {
		t.Fatal(err)
	}
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if info.Name() != webRulesFile || info.Size() != 3 || info.IsDir() || info.Mode() != 0o444 || !info.ModTime().IsZero() || info.Sys() != nil {
		t.Errorf("replaced file info %s %d %v %v", info.Name(), info.Size(), info.IsDir(), info.Mode())
	}
	data, err := io.ReadAll(file)
	if err != nil || string(data) != "{}\n" || file.Close() != nil {
		t.Errorf("replaced file reads %q: %v", data, err)
	}
	if _, err := fs.ReadFile(tree, "parity/"+eslintConfigFile); err != nil {
		t.Errorf("a base file is unreadable through the overlay: %v", err)
	}
	if _, err := tree.Open("parity/absent.mjs"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a missing file answers %v, want fs.ErrNotExist", err)
	}
}

// The snapshot's S125 fires through the overlaid parity set.
func TestWebOverlayIsProven(t *testing.T) {
	t.Parallel()
	policy := &model.EffectivePolicy{ProfileData: &model.ProfileData{SonarWay: webProfileWay()}}
	proveWebSet(t, "testdata/web/overlay.yml", model.SurfaceParity, webDriversFor(policy))
}

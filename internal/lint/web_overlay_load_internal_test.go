package lint

import (
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

// TestWebOverlayOfEveryKeyLoads activates every key of the index for both
// script languages: whatever a server activates, ESLint must still accept
// the overlaid configuration, because a rule it refuses would leave the
// whole run unmeasured.
func TestWebOverlayOfEveryKeyLoads(t *testing.T) {
	t.Parallel()
	idx, err := readSonarIndex()
	if err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(idx.Rules))
	for key := range idx.Rules {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	way := webWay()
	for _, key := range keys {
		way.Rules = append(way.Rules, model.SonarWayRule{Language: "js", Key: key}, model.SonarWayRule{Language: "ts", Key: key})
	}
	drivers := webDriversFor(&model.EffectivePolicy{ProfileData: &model.ProfileData{SonarWay: way}})
	eslint := drivers[slices.IndexFunc(drivers, func(d Driver) bool { return d.Tool == eslintTool })]
	path := webToolPath(t, eslintTool)
	files := map[string][]byte{
		"src/total.ts":  []byte("export function total(rows: number[]): number {\n  return rows.length;\n}\n"),
		"src/panel.jsx": []byte("export function Panel() {\n  return <p>total</p>;\n}\n"),
	}
	for _, set := range webSets {
		webIssues(t, eslint, path, webMaterialize(t, eslint, set), set, files)
	}
}

// TestWebOverlayManifestIsBound proves the overlay entry and its trees with
// no tool installed, as the other web manifests are.
func TestWebOverlayManifestIsBound(t *testing.T) {
	t.Parallel()
	webManifestBound(t, "testdata/web/overlay.yml")
}

package lint

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// sonarIndexFile is the Sonar key index tools/sonarmap writes at the root
// of the web configuration tree.
const sonarIndexFile = "sonar-index.json"

// sonarIndex maps a Sonar key without its language to the ESLint rules the
// pinned eslint-plugin-sonarjs README gives it, the shape tools/sonarmap
// writes.
type sonarIndex struct {
	Plugin  string              `json:"plugin"`
	Version string              `json:"version"`
	Rules   map[string][]string `json:"rules"`
}

// readSonarIndex reads the committed index from the embedded tree.
func readSonarIndex() (sonarIndex, error) {
	data, err := fs.ReadFile(webConfig(), sonarIndexFile)
	if err != nil {
		return sonarIndex{}, fmt.Errorf("lint: read the Sonar key index: %w", err)
	}
	var idx sonarIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return sonarIndex{}, fmt.Errorf("lint: read the Sonar key index: %w", err)
	}
	return idx, nil
}

// snapshotLanguage answers the language key a snapshot uses for the
// language of a rule map row: the map writes javascript where the server
// writes js.
func snapshotLanguage(rowLanguage string) string {
	switch rowLanguage {
	case "javascript":
		return "js"
	case "typescript":
		return "ts"
	default:
		return rowLanguage
	}
}

// rowLanguage is the inverse of snapshotLanguage.
func rowLanguage(snapshot string) string {
	switch snapshot {
	case "js":
		return "javascript"
	case "ts":
		return "typescript"
	default:
		return snapshot
	}
}

// pinnedWebRule says whether a rule the index names can run: a core rule,
// or a rule of a plugin the kit pins. A rule of a plugin the kit does not
// install, or one the pinned plugin no longer ships under the README's
// name, would make ESLint refuse the whole configuration, so it never joins.
func pinnedWebRule(name string) bool {
	if slices.Contains(unshippedWebRules(), name) {
		return false
	}
	plugin, _, found := strings.Cut(name, "/")
	return !found || slices.Contains([]string{"sonarjs", "@typescript-eslint", "unicorn", "react", "jsx-a11y"}, plugin)
}

// unshippedWebRules are the rules the pinned sonarjs README names that the
// pinned plugins do not define: unicorn 78 renamed no-array-for-each to
// no-for-each, and the README still writes the old name.
func unshippedWebRules() []string { return []string{"unicorn/no-array-for-each"} }

// activeKeys answers the keys the snapshot activates, by language.
func activeKeys(way *model.SonarWay) map[string]map[string]bool {
	active := map[string]map[string]bool{}
	for _, r := range way.Rules {
		if active[r.Language] == nil {
			active[r.Language] = map[string]bool{}
		}
		active[r.Language][r.Key] = true
	}
	return active
}

// keptRows keeps a row when the snapshot covers no rule of its language,
// since the server then says nothing about it, or activates its key.
func keptRows(rows []Row, active map[string]map[string]bool) []Row {
	var out []Row
	for _, r := range rows {
		language, key, _ := strings.Cut(r.Sonar, ":")
		keys, covered := active[snapshotLanguage(language)]
		if !covered || keys[key] {
			out = append(out, r)
		}
	}
	return out
}

// webOverlay answers the public rows as the snapshot governs them: the rows
// of a language it covers follow its activation, and each script key it
// activates that the map leaves out joins through the index. A rule's
// parameters are not translated: the joined rule runs at its defaults.
func webOverlay(rows []Row, way *model.SonarWay, idx sonarIndex) []Row {
	if way == nil {
		return rows
	}
	out := keptRows(rows, activeKeys(way))
	for _, r := range way.Rules {
		if r.Language != "js" && r.Language != "ts" {
			continue
		}
		for _, name := range idx.Rules[r.Key] {
			named := slices.ContainsFunc(out, func(row Row) bool { return row.Rule == name })
			if named || !pinnedWebRule(name) {
				continue
			}
			out = append(out, Row{
				Sonar: rowLanguage(r.Language) + ":" + r.Key, Tool: eslintTool, Rule: name,
				Where: []string{"L"}, Match: "exact", Set: model.SurfaceParity,
			})
		}
	}
	return out
}

// webDriversFor answers the web drivers of a policy: the public ones when
// its profile pins no snapshot, otherwise drivers whose tree carries the
// overlaid rules in both sets. The embedded index and the rendering are
// proven by the tests, so their failure answers the public drivers and
// never ships.
func webDriversFor(policy *model.EffectivePolicy) []Driver {
	drivers := webDrivers()
	if policy == nil || policy.ProfileData == nil || policy.ProfileData.SonarWay == nil {
		return drivers
	}
	idx, err := readSonarIndex()
	if err != nil {
		return drivers
	}
	rules, err := renderWebRules(webOverlay(webRows(), policy.ProfileData.SonarWay, idx))
	if err != nil {
		return drivers
	}
	tree := replacedFS{base: webConfig(), files: map[string][]byte{}}
	for _, set := range []string{model.SurfaceParity, model.SurfaceStrict} {
		tree.files[set+"/"+webRulesFile] = rules
	}
	for i := range drivers {
		drivers[i].Config = tree
	}
	return drivers
}

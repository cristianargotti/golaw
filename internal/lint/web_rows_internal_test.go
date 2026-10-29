package lint

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

var updateWeb = flag.Bool("update-web", false, "rewrite the rendered web rules of both sets from rulemap/web.yml")

const webManifest = "proof/web.yml"

// webRowCase is one row of the rule map a task claims.
type webRowCase struct {
	sonar, tool, rule, match string
}

// webEntryID is the proof id of a row: lint.<tool>-<finding code>.
func webEntryID(r Row) string {
	return "lint." + r.Tool + "-" + Code(r.Rule)
}

// assertWebRows proves the claimed rows are in the map with their match and
// have a proof entry.
func assertWebRows(t *testing.T, cases []webRowCase) {
	t.Helper()
	m, err := proof.LoadManifest(webManifest)
	if err != nil {
		t.Fatal(err)
	}
	rows := webRows()
	for _, c := range cases {
		i := slices.IndexFunc(rows, func(r Row) bool { return r.Sonar == c.sonar && r.Tool == c.tool && r.Rule == c.rule })
		if i < 0 {
			t.Errorf("rulemap/web.yml holds no row %s %s %s", c.sonar, c.tool, c.rule)
			continue
		}
		if rows[i].Match != c.match {
			t.Errorf("row %s %s matches %s, want %s", c.sonar, c.rule, rows[i].Match, c.match)
		}
		if _, ok := m.Lookup(webEntryID(rows[i])); !ok {
			t.Errorf("%s has no entry %s", webManifest, webEntryID(rows[i]))
		}
	}
}

func TestWebRowsAreValid(t *testing.T) {
	t.Parallel()
	rows := webRows()
	if len(rows) == 0 {
		t.Fatal("rulemap/web.yml decodes to no row")
	}
	seen := map[string]bool{}
	for _, r := range rows {
		if r.Tool != eslintTool && r.Tool != stylelintTool {
			t.Errorf("row %s names the tool %s, which is no web driver", r.Sonar, r.Tool)
		}
		if !slices.Equal(r.Where, []string{"L"}) {
			t.Errorf("row %s runs in %v; a web tool runs in the lint target only", r.Sonar, r.Where)
		}
		if r.Set != model.SurfaceParity {
			t.Errorf("row %s enables at %s; every mapped rule is in the parity set", r.Sonar, r.Set)
		}
		language, _, _ := strings.Cut(r.Sonar, ":")
		if !slices.Contains([]string{"javascript", "typescript", "css"}, language) {
			t.Errorf("row %s is not a web Sonar key", r.Sonar)
		}
		if seen[r.Tool+" "+r.Rule] {
			t.Errorf("rule %s %s is mapped twice", r.Tool, r.Rule)
		}
		seen[r.Tool+" "+r.Rule] = true
	}
}

func TestWebRowsAreProven(t *testing.T) {
	t.Parallel()
	m, err := proof.LoadManifest(webManifest)
	if err != nil {
		t.Fatal(err)
	}
	rows := webRows()
	for _, r := range rows {
		e, ok := m.Lookup(webEntryID(r))
		if !ok {
			t.Errorf("row %s %s has no proof entry %s", r.Sonar, r.Rule, webEntryID(r))
			continue
		}
		if e.Code != Code(r.Rule) {
			t.Errorf("entry %s carries the code %s, want %s", e.ID, e.Code, Code(r.Rule))
		}
	}
	if len(m.Checks) != len(rows) {
		t.Errorf("%s holds %d entries for %d rows", webManifest, len(m.Checks), len(rows))
	}
	webManifestBound(t, webManifest)
}

func TestWebRulesAreRendered(t *testing.T) {
	t.Parallel()
	want, err := renderWebRules(webRows())
	if err != nil {
		t.Fatal(err)
	}
	for _, set := range webSets {
		name := filepath.Join("config", "web", set, webRulesFile)
		if *updateWeb {
			if err := os.WriteFile(name, want, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if webFile(t, set, webRulesFile) != string(want) {
			t.Errorf("%s is stale: run go test ./internal/lint -run TestWebRulesAreRendered -update-web", name)
		}
	}
}

func TestWebRenderSplitsTheRules(t *testing.T) {
	t.Parallel()
	rows := []Row{
		{Sonar: "javascript:S6479", Tool: eslintTool, Rule: "react/no-array-index-key"},
		{Sonar: "javascript:S6819", Tool: eslintTool, Rule: "jsx-a11y/prefer-tag-over-role"},
		{Sonar: "javascript:S3776", Tool: eslintTool, Rule: "sonarjs/cognitive-complexity"},
		{Sonar: "javascript:S2681", Tool: eslintTool, Rule: "sonarjs/no-unenclosed-multiline-block"},
		{Sonar: "css:S4666", Tool: stylelintTool, Rule: "no-duplicate-selectors"},
	}
	data, err := renderWebRules(rows)
	if err != nil {
		t.Fatal(err)
	}
	var got webRules
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	want := webRules{
		ESLint: webESLintRules{
			JSX: map[string]any{"jsx-a11y/prefer-tag-over-role": "error", "react/no-array-index-key": "error"},
			Script: map[string]any{
				"sonarjs/cognitive-complexity":          []any{"error", float64(sonarCognitiveThreshold)},
				"sonarjs/no-unenclosed-multiline-block": "error",
			},
		},
		Stylelint: map[string]any{"no-duplicate-selectors": true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("rendered %+v, want %+v", got, want)
	}
	if !strings.HasSuffix(string(data), "}\n") {
		t.Error("the rendered rules end without a newline")
	}
}

func TestWebDrivers(t *testing.T) {
	t.Parallel()
	drivers := webDrivers()
	tools := make([]string, 0, len(drivers))
	for _, d := range drivers {
		tools = append(tools, d.Tool)
	}
	if !slices.Equal(tools, []string{"eslint", "stylelint"}) {
		t.Errorf("webDrivers = %v, want eslint then stylelint", tools)
	}
}

// The proof of every row of the public map through the shared harness.
func TestWebParityRowsAreProven(t *testing.T) {
	t.Parallel()
	proveManifest(t, webManifest, webDrivers())
}

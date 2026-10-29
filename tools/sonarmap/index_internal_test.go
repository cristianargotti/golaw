package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/lint"
)

// pinnedVersion is the eslint-plugin-sonarjs pin of M6.4.2.
const pinnedVersion = "4.2.2"

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestParseIndexReadsTheThreeTables(t *testing.T) {
	t.Parallel()
	idx, err := parseIndex(readFixture(t, "README.md"), pinnedVersion)
	if err != nil {
		t.Fatal(err)
	}
	if idx.Plugin != "eslint-plugin-sonarjs" || idx.Version != pinnedVersion {
		t.Errorf("index of %s %s", idx.Plugin, idx.Version)
	}
	cases := []struct {
		key  string
		want []string
	}{
		{"S1523", []string{"sonarjs/code-eval"}},
		{"S2681", []string{"sonarjs/no-unenclosed-multiline-block"}},
		{"S103", []string{"@stylistic/max-len"}},
		{"S121", []string{"curly"}},
		{"S7764", []string{"unicorn/prefer-global-this"}},
		{"S1186", []string{"@typescript-eslint/no-empty-function"}},
		{"S6479", []string{"react/no-array-index-key"}},
	}
	for _, c := range cases {
		if got := idx.Rules[c.key]; !slices.Equal(got, c.want) {
			t.Errorf("%s maps to %v, want %v", c.key, got, c.want)
		}
	}
	if len(idx.Rules) != len(cases) {
		t.Errorf("the index holds %d keys, want %d", len(idx.Rules), len(cases))
	}
}

func TestParseIndexRefusesAnUnknownLayout(t *testing.T) {
	t.Parallel()
	readme := string(readFixture(t, "README.md"))
	external, _, _ := strings.Cut(readme, "## Improved ESLint rules")
	native, _, _ := strings.Cut(readme, "## ESLint rules")
	cases := map[string]string{
		"no rule of the rules table": "# eslint-plugin-sonarjs\n",
		"no external table":          native,
		"no decorated table":         external,
	}
	for name, text := range cases {
		if _, err := parseIndex([]byte(text), pinnedVersion); !errors.Is(err, errReadme) {
			t.Errorf("%s: err = %v, want errReadme", name, err)
		}
	}
}

func TestVerifyRowsNamesTheRowTheReadmeContradicts(t *testing.T) {
	t.Parallel()
	idx := index{Version: pinnedVersion, Rules: map[string][]string{"S2681": {"sonarjs/no-unenclosed-multiline-block"}}}
	rows := []lint.Row{
		{Sonar: "javascript:S2681", Tool: "eslint", Rule: "sonarjs/no-unenclosed-multiline-block"},
		{Sonar: "javascript:S3358", Tool: "eslint", Rule: "sonarjs/no-nested-conditional"},
		{Sonar: "css:S4666", Tool: "stylelint", Rule: "no-duplicate-selectors"},
	}
	got := verifyRows(idx, rows)
	if len(got) != 1 || !strings.HasPrefix(got[0], "javascript:S3358 sonarjs/no-nested-conditional: ") {
		t.Errorf("verifyRows = %q, want one line for the S3358 row", got)
	}
}

func TestRenderIsStable(t *testing.T) {
	t.Parallel()
	idx := index{Plugin: "eslint-plugin-sonarjs", Version: pinnedVersion, Rules: map[string][]string{"S121": {"curly"}, "S1523": {"sonarjs/code-eval"}}}
	first, err := render(idx)
	if err != nil {
		t.Fatal(err)
	}
	second, err := render(idx)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) || !strings.HasSuffix(string(first), "}\n") {
		t.Errorf("render is not stable or ends without a newline:\n%s", first)
	}
	var back index
	if err := json.Unmarshal(first, &back); err != nil || !slices.Equal(back.Rules["S121"], []string{"curly"}) {
		t.Errorf("the rendered index does not read back: %v", err)
	}
}

// The committed index agrees with every ESLint row of the public map, with
// no tool installed.
func TestCommittedIndexCoversTheRuleMap(t *testing.T) {
	t.Parallel()
	var idx index
	if err := json.Unmarshal(readRepo(t, "internal/lint/config/web/sonar-index.json"), &idx); err != nil {
		t.Fatal(err)
	}
	if idx.Version != pinnedVersion {
		t.Errorf("the committed index reads eslint-plugin-sonarjs %s, the pin is %s", idx.Version, pinnedVersion)
	}
	rows, err := lint.DecodeRows(readRepo(t, "internal/lint/rulemap/web.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if bad := verifyRows(idx, rows); len(bad) != 0 {
		t.Errorf("the public map disagrees with the pinned README:\n%s", strings.Join(bad, "\n"))
	}
}

func readRepo(t *testing.T, rel string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

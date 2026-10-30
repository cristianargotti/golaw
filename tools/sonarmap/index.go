// Command sonarmap reads the pinned eslint-plugin-sonarjs, its rules table
// and the README's external and decorated tables, into the Sonar key index
// the public rule map is verified against and a profile snapshot is mapped
// through. It reads the installed package alone and never touches the
// network, so the index changes only when the pin does.
package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/lint"
)

// pluginName is the package the index is read from.
const pluginName = "eslint-plugin-sonarjs"

// The README rows the index reads: a rule of the plugin links its rspec
// page, and a row of the external and decorated tables names the key and
// links each rule that implements it.
var (
	ruleRow  = regexp.MustCompile(`^\|\s*\[([a-z0-9-]+)\]\(https://sonarsource\.github\.io/rspec/#/rspec/(S\d+)/javascript\)`)
	tableRow = regexp.MustCompile(`^\|\s*(S\d+)\s*\|(.*)\|\s*$`)
	linkName = regexp.MustCompile(`\[([^\]]+)\]\(`)
)

// index maps a Sonar key without its language to the ESLint rules that
// stand in for it, as the pinned README gives them.
type index struct {
	Plugin  string              `json:"plugin"`
	Version string              `json:"version"`
	Rules   map[string][]string `json:"rules"`
}

// readmeTables names the two marked tables of rules from other plugins.
func readmeTables() []string { return []string{"external rules", "decorated rules"} }

// tableRegion answers the text between the markers of a table, so a row of
// another table never lands under the wrong heading.
func tableRegion(text, table string) (string, error) {
	_, rest, found := strings.Cut(text, "<!--- start "+table+" -->")
	if !found {
		return "", fmt.Errorf("%w: no start marker of the %s table", errReadme, table)
	}
	region, _, found := strings.Cut(rest, "<!--- end "+table+" -->")
	if !found {
		return "", fmt.Errorf("%w: no end marker of the %s table", errReadme, table)
	}
	return region, nil
}

// eslintName turns the README's plugin path into the name ESLint reports:
// the README writes the npm package where the configuration writes the
// plugin prefix.
func eslintName(name string) string {
	for _, p := range [][2]string{{"eslint/", ""}, {"typescript-eslint/", "@typescript-eslint/"}, {"@stylistic/eslint-plugin/", "@stylistic/"}} {
		if rest, found := strings.CutPrefix(name, p[0]); found {
			return p[1] + rest
		}
	}
	return name
}

// add keeps each list sorted and unique, so the rendering is stable.
func add(rules map[string][]string, key, name string) {
	list := rules[key]
	if slices.Contains(list, name) {
		return
	}
	list = append(list, name)
	slices.Sort(list)
	rules[key] = list
}

// parseIndex reads the rules table and both marked tables of the README.
func parseIndex(readme []byte, version string) (index, error) {
	text := string(readme)
	idx := index{Plugin: pluginName, Version: version, Rules: map[string][]string{}}
	for line := range strings.Lines(text) {
		if m := ruleRow.FindStringSubmatch(line); m != nil {
			add(idx.Rules, m[2], "sonarjs/"+m[1])
		}
	}
	if len(idx.Rules) == 0 {
		return index{}, fmt.Errorf("%w: no row of the rules table", errReadme)
	}
	for _, table := range readmeTables() {
		region, err := tableRegion(text, table)
		if err != nil {
			return index{}, err
		}
		for line := range strings.Lines(region) {
			m := tableRow.FindStringSubmatch(strings.TrimRight(line, "\r\n"))
			if m == nil {
				continue
			}
			for _, link := range linkName.FindAllStringSubmatch(m[2], -1) {
				add(idx.Rules, m[1], eslintName(link[1]))
			}
		}
	}
	return idx, nil
}

// verifyRows answers one line per ESLint row whose rule the README does not
// give its key; stylelint rows are not indexed.
func verifyRows(idx index, rows []lint.Row) []string {
	var out []string
	for _, r := range rows {
		if r.Tool != "eslint" {
			continue
		}
		_, key, _ := strings.Cut(r.Sonar, ":")
		rules := idx.Rules[key]
		if slices.Contains(rules, r.Rule) {
			continue
		}
		mapped := "no rule"
		if len(rules) > 0 {
			mapped = strings.Join(rules, ", ")
		}
		out = append(out, fmt.Sprintf("%s %s: the README of %s %s maps %s to %s", r.Sonar, r.Rule, pluginName, idx.Version, key, mapped))
	}
	return out
}

// render answers the index as the committed asset holds it.
func render(idx index) ([]byte, error) {
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("sonarmap: render: %w", err)
	}
	return append(data, '\n'), nil
}

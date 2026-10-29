package lint

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"
)

// webEmbedded holds the kit's own web configurations, so a repository file
// never weakens a set nor runs code at lint time (M6.4.1).
//
//go:embed config/web
var webEmbedded embed.FS

// The web configuration tree and the names of its files in each set.
const (
	webConfigRoot       = "config/web"
	eslintConfigFile    = "eslint.config.mjs"
	stylelintConfigFile = "stylelint.config.mjs"
	stylelintIgnoreFile = "golaw.stylelintignore"
	webRulesFile        = "web-rules.json"
	tsconfigFile        = "tsconfig.golaw.json"
)

// webConfig answers the tree with parity/ and strict/ at its root.
func webConfig() fs.FS {
	tree, err := fs.Sub(webEmbedded, webConfigRoot)
	if err != nil {
		// fs.Sub fails only on an invalid path, and the root is a constant.
		return webEmbedded
	}
	return tree
}

// sonarCognitiveThreshold is the Sonar way threshold of S3776, so the
// parity set reports the function the pipeline reports and no other.
const sonarCognitiveThreshold = 15

// webRules is the shape golaw-web.mjs and the stylelint configurations read
// from web-rules.json.
type webRules struct {
	ESLint    webESLintRules `json:"eslint"`
	Stylelint map[string]any `json:"stylelint"`
}

// webESLintRules splits the ESLint rules by the files they run on.
type webESLintRules struct {
	JSX    map[string]any `json:"jsx"`
	Script map[string]any `json:"script"`
}

// webRuleOptions are the options a mapped rule runs with in place of its
// default, so a rule reports at the Sonar way threshold.
func webRuleOptions() map[string][]any {
	return map[string][]any{"sonarjs/cognitive-complexity": {sonarCognitiveThreshold}}
}

// isJSXRule says whether a rule reads JSX alone: those run on the JSX and
// TSX files only, where the plugins find what they report.
func isJSXRule(rule string) bool {
	return strings.HasPrefix(rule, "react/") || strings.HasPrefix(rule, "jsx-a11y/")
}

// eslintSetting answers the ESLint setting of a mapped rule at error.
func eslintSetting(rule string) any {
	options, ok := webRuleOptions()[rule]
	if !ok {
		return "error"
	}
	return append([]any{"error"}, options...)
}

// renderWebRules renders the rows into web-rules.json, the file both sets
// read, so the configurations enable exactly the rules of the public map.
func renderWebRules(rows []Row) ([]byte, error) {
	out := webRules{
		ESLint:    webESLintRules{JSX: map[string]any{}, Script: map[string]any{}},
		Stylelint: map[string]any{},
	}
	for _, r := range rows {
		switch {
		case r.Tool == stylelintTool:
			out.Stylelint[r.Rule] = true
		case r.Tool == eslintTool && isJSXRule(r.Rule):
			out.ESLint.JSX[r.Rule] = eslintSetting(r.Rule)
		case r.Tool == eslintTool:
			out.ESLint.Script[r.Rule] = eslintSetting(r.Rule)
		}
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("lint: render the web rules: %w", err)
	}
	return append(data, '\n'), nil
}

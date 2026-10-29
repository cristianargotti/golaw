package lint

import (
	"encoding/json"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/surface"
)

// webSets are the two rule sets of M6.4.1, in floor order.
var webSets = []string{model.SurfaceParity, model.SurfaceStrict}

// webFile reads one file of a set from the embedded tree.
func webFile(t *testing.T, set, name string) string {
	t.Helper()
	data, err := fs.ReadFile(webConfig(), set+"/"+name)
	if err != nil {
		t.Fatalf("the %s web set holds no %s: %v", set, name, err)
	}
	return string(data)
}

func TestWebConfigHoldsBothSets(t *testing.T) {
	t.Parallel()
	names := []string{eslintConfigFile, stylelintConfigFile, stylelintIgnoreFile, "golaw-load.mjs", "golaw-web.mjs", webRulesFile, tsconfigFile}
	for _, set := range webSets {
		for _, name := range names {
			if strings.TrimSpace(webFile(t, set, name)) == "" {
				t.Errorf("%s/%s is empty", set, name)
			}
		}
	}
}

func TestWebSetsShareTheFrame(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"golaw-load.mjs", "golaw-web.mjs", webRulesFile, stylelintIgnoreFile} {
		if webFile(t, model.SurfaceParity, name) != webFile(t, model.SurfaceStrict, name) {
			t.Errorf("%s differs between the parity and the strict set", name)
		}
	}
}

func webCompilerOptions(t *testing.T, set string) map[string]any {
	t.Helper()
	var config struct {
		CompilerOptions map[string]any `json:"compilerOptions"`
		Include         []string       `json:"include"`
	}
	if err := json.Unmarshal([]byte(webFile(t, set, tsconfigFile)), &config); err != nil {
		t.Fatalf("%s/%s: %v", set, tsconfigFile, err)
	}
	if !slices.Equal(config.Include, []string{"**/*"}) {
		t.Errorf("%s program includes %v, want every script under the root", set, config.Include)
	}
	return config.CompilerOptions
}

func TestWebTSConfigOptions(t *testing.T) {
	t.Parallel()
	parity := webCompilerOptions(t, model.SurfaceParity)
	strict := webCompilerOptions(t, model.SurfaceStrict)
	for _, option := range []string{"strict", "allowJs", "noEmit", "skipLibCheck"} {
		if parity[option] != true || strict[option] != true {
			t.Errorf("%s is not on in both programs", option)
		}
	}
	strictOnly := []string{
		"noUncheckedIndexedAccess", "exactOptionalPropertyTypes", "noImplicitOverride",
		"noPropertyAccessFromIndexSignature", "noFallthroughCasesInSwitch", "useUnknownInCatchVariables", "checkJs",
	}
	for _, option := range strictOnly {
		if strict[option] != true {
			t.Errorf("the strict program leaves %s off", option)
		}
		if parity[option] == true {
			t.Errorf("the parity program turns %s on, beyond what the analyzer reads", option)
		}
	}
}

func TestWebConfigNamesEachSet(t *testing.T) {
	t.Parallel()
	cases := []struct {
		set, file, token string
		want             bool
	}{
		{model.SurfaceStrict, eslintConfigFile, "strictTypeChecked", true},
		{model.SurfaceStrict, eslintConfigFile, "stylisticTypeChecked", true},
		{model.SurfaceStrict, eslintConfigFile, "frame.unicorn.configs.recommended.rules", true},
		{model.SurfaceStrict, eslintConfigFile, "frame.jsxA11y.flatConfigs.strict.rules", true},
		{model.SurfaceStrict, eslintConfigFile, "...security.configs.recommended.rules", true},
		{model.SurfaceStrict, eslintConfigFile, "'security/detect-object-injection': 'off'", true},
		{model.SurfaceStrict, eslintConfigFile, "curly: ['error', 'all']", true},
		{model.SurfaceStrict, eslintConfigFile, "'max-lines-per-function'", true},
		{model.SurfaceStrict, eslintConfigFile, "'@html-eslint/require-lang': 'error'", true},
		{model.SurfaceStrict, eslintConfigFile, "'@html-eslint/require-img-alt': 'error'", true},
		{model.SurfaceStrict, eslintConfigFile, "...frame.parity,\n  ...frame.closing,\n]", true},
		{model.SurfaceParity, eslintConfigFile, "curly", false},
		{model.SurfaceParity, eslintConfigFile, "TypeChecked", false},
		{model.SurfaceParity, eslintConfigFile, "@html-eslint", false},
		{model.SurfaceParity, eslintConfigFile, "security", false},
		{model.SurfaceParity, "golaw-web.mjs", "sonarjs.configs.recommended", true},
		{model.SurfaceParity, "golaw-web.mjs", "parserOptions: { programs: [program(ts)] }", true},
		{model.SurfaceParity, "golaw-web.mjs", "plugins: { html }", true},
		{model.SurfaceParity, "golaw-web.mjs", "disableTypeChecked, files: HTML_FILES", true},
		{model.SurfaceStrict, stylelintConfigFile, "'stylelint-config-standard'", true},
		{model.SurfaceParity, stylelintConfigFile, "stylelint-config-standard", false},
		{model.SurfaceParity, stylelintConfigFile, "customSyntax: postcssHtml", true},
		{model.SurfaceStrict, stylelintConfigFile, "customSyntax: postcssHtml", true},
	}
	for _, c := range cases {
		if got := strings.Contains(webFile(t, c.set, c.file), c.token); got != c.want {
			t.Errorf("%s/%s holds %q: %v, want %v", c.set, c.file, c.token, got, c.want)
		}
	}
}

func TestWebArgs(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("tools", "config", "0.3.0", model.SurfaceStrict)
	in := Input{Root: "/repo", Files: []string{"views/panel.html", "views/panel.js"}, ConfigDir: dir, Set: model.SurfaceStrict}
	withGlobals := in
	withGlobals.Globals = []string{"PANEL_STATE", "renderPanel"}
	eslintHead := []string{"--config", filepath.Join(dir, "eslint.config.mjs"), "--format", "json", "--no-warn-ignored", "--no-error-on-unmatched-pattern"}
	cases := []struct {
		name string
		args func(Input) []string
		in   Input
		want []string
	}{
		{"eslint", eslintArgs, in, append(slices.Clone(eslintHead), "views/panel.html", "views/panel.js")},
		{"eslint with globals", eslintArgs, withGlobals, append(slices.Clone(eslintHead), "--global", "PANEL_STATE,renderPanel", "views/panel.html", "views/panel.js")},
		{"stylelint", stylelintArgs, in, []string{
			"--config", filepath.Join(dir, "stylelint.config.mjs"), "--ignore-path", filepath.Join(dir, "golaw.stylelintignore"),
			"--formatter", "json", "--allow-empty-input", "views/panel.html", "views/panel.js",
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := c.args(c.in); !slices.Equal(got, c.want) {
				t.Errorf("args = %q, want %q", got, c.want)
			}
		})
	}
}

func TestWebDriversStayOutOfTheHook(t *testing.T) {
	t.Parallel()
	cases := []struct {
		driver    Driver
		tool      string
		format    Format
		languages []surface.Language
	}{
		{eslintDriver(), "eslint", FormatESLint, []surface.Language{surface.JavaScript, surface.TypeScript, surface.JSX, surface.TSX, surface.HTML}},
		{stylelintDriver(), "stylelint", FormatStylelint, []surface.Language{surface.CSS, surface.HTML}},
	}
	for _, c := range cases {
		d := c.driver
		if d.Tool != c.tool || d.Format != c.format || !slices.Equal(d.Languages, c.languages) {
			t.Errorf("driver %s = %s %s %v, want %s %s %v", c.tool, d.Tool, d.Format, d.Languages, c.tool, c.format, c.languages)
		}
		if d.HookArgs != nil {
			t.Errorf("%s may run in the hook; M6.6.1 keeps it in the lint target", d.Tool)
		}
		if d.Config == nil || d.Args == nil {
			t.Errorf("%s carries no configuration or no arguments", d.Tool)
		}
	}
}

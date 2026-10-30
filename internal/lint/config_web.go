package lint

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"time"
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

// memFileMode is the mode of a replaced file: read only, as the embedded
// files it stands in for.
const memFileMode fs.FileMode = 0o444

// replacedFS answers some files of a base tree with other bytes, so a
// profile's rules replace web-rules.json while every other file stays the
// kit's. A directory listing comes from the base, so its sizes are the
// base's: toolchain.Materialize reads each file through fs.ReadFile and
// never trusts a listed size.
type replacedFS struct {
	base  fs.FS
	files map[string][]byte
}

// Open answers a replaced path from memory and passes the rest to the base.
func (r replacedFS) Open(name string) (fs.File, error) {
	if data, ok := r.files[name]; ok {
		return &memFile{Reader: bytes.NewReader(data), info: memInfo{name: path.Base(name), size: int64(len(data))}}, nil
	}
	file, err := r.base.Open(name)
	if err != nil {
		return nil, fmt.Errorf("lint: open %s: %w", name, err)
	}
	return file, nil
}

// memFile is one replaced file, open for reading.
type memFile struct {
	*bytes.Reader

	info memInfo
}

// Stat answers the replaced file's name and size.
func (f *memFile) Stat() (fs.FileInfo, error) { return f.info, nil }

// Close has nothing to release: the bytes stay in the tree.
func (f *memFile) Close() error { return nil }

// memInfo describes a replaced file.
type memInfo struct {
	name string
	size int64
}

func (i memInfo) Name() string { return i.name }

func (i memInfo) Size() int64 { return i.size }

func (memInfo) Mode() fs.FileMode { return memFileMode }

// ModTime is the zero time: the embedded files carry none either.
func (memInfo) ModTime() time.Time { return time.Time{} }

func (memInfo) IsDir() bool { return false }

func (memInfo) Sys() any { return nil }

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

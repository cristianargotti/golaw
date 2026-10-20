package assets_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/runner"
)

func templateValues() assets.Values {
	tools := make(map[string]string)
	for _, tool := range runner.Toolchain() {
		tools[tool.Name] = tool.Version
	}
	return assets.Values{
		ModulePath: "example.test/service",
		GoVersion:  "1.27", KitVersion: "0.2.0", Language: "pt-BR",
		Capabilities: []string{"go"}, Decisions: "docs/adr",
		Main: "root", Tools: tools,
	}
}

// renderCase is one template the renderer reads: where it lands, its text,
// the decisions directory it renders with and whether it must fail.
type renderCase struct {
	name, destination, source, decisions string
	fail                                 bool
}

func TestRenderParsesAndExhaustsPlaceholders(t *testing.T) {
	t.Parallel()
	cases := []renderCase{
		{"yaml", "a.yml", "version: {{.KitVersion}}\n", "docs/adr", false},
		{"json", "a.json", "{\"version\":\"{{.KitVersion}}\"}\n", "docs/adr", false},
		{"go", "a.go", "package sample\nconst Version = \"{{.KitVersion}}\"\n", "docs/adr", false},
		{"document", "{{.Decisions}}/record.md", "# Record\n", "docs/decisions", false},
		{"missing field", "a.yml", "value: {{.Absent}}\n", "docs/adr", true},
		{"missing tool", "a.yml", "value: {{.Tools.absent}}\n", "docs/adr", true},
		{"missing indexed tool", "a.yml", "value: {{index .Tools \"absent\"}}\n", "docs/adr", true},
		{"capability function", "a.yml", "enabled: {{has .Capabilities \"go\"}}\n", "docs/adr", false},
		{"prefix function", "a.yml", "enabled: {{hasPrefix .ModulePath \"example.test/\"}}\n", "docs/adr", false},
		{"empty required value", "{{.Decisions}}/record.md", "# Record\n", "", true},
		{"remaining expression", "a.yml", "{{print \"{{.Absent}}\"}}\n", "docs/adr", true},
		{"bad yaml", "a.yml", "value: [\n", "docs/adr", true},
		{"bad json", "a.json", "{\"value\":\n", "docs/adr", true},
		{"bad go", "a.go", "package\n", "docs/adr", true},
		{"escaped destination", "{{.Decisions}}/record.md", "# Record\n", "../outside", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			requireRender(t, tc)
		})
	}
}

// requireRender renders one case twice: a failing case answers ErrRender,
// any other the same bytes, their digest and its destination with every
// placeholder exhausted.
func requireRender(t *testing.T, tc renderCase) {
	t.Helper()
	tree := fstest.MapFS{
		"templates/input.tmpl": {Data: []byte(tc.source)},
	}
	entry := assets.Template{
		Source:      "templates/input.tmpl",
		Destination: tc.destination, Ownership: assets.OwnedByKit,
	}
	values := templateValues()
	values.Decisions = tc.decisions
	first, err := assets.Render(tree, entry, values)
	if tc.fail {
		if !errors.Is(err, assets.ErrRender) {
			t.Fatalf("expected render error, got %v", err)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	second, err := assets.Render(tree, entry, values)
	if err != nil || !bytes.Equal(first.Data, second.Data) {
		t.Fatalf("nondeterministic render: %v", err)
	}
	if first.SHA256 != install.Digest(first.Data) {
		t.Fatalf("digest = %q", first.SHA256)
	}
	want := strings.ReplaceAll(tc.destination, "{{.Decisions}}", tc.decisions)
	if first.Path != want || strings.Contains(string(first.Data), "{{") {
		t.Fatalf("rendered = %+v", first)
	}
}

func TestRenderValidatesSyntaxAndTemplateTrees(t *testing.T) {
	t.Parallel()
	cases := []renderCase{
		{"template syntax", "a.txt", "{{", "docs/adr", true},
		{"unknown function", "a.txt", "{{unknown .Main}}", "docs/adr", true},
		{"closing delimiter", "a.txt", "left }}", "docs/adr", true},
		{"duplicate YAML", "a.yaml", "a: 1\na: 2\n", "docs/adr", true},
		{"YAML stream", "a.yaml", "a: 1\n---\nb: 2\n", "docs/adr", false},
		{"broken YAML tail", "a.yaml", "a: 1\n---\nb: [", "docs/adr", true},
		{"empty YAML", "a.yaml", "", "docs/adr", false},
		{"two JSON values", "a.json", "{} {}", "docs/adr", true},
		{"JSON null", "a.json", "null", "docs/adr", false},
		{"Go syntax only", "a.go", "package sample\nvar Value = unresolved\n", "docs/adr", false},
		{"indexed tool", "a.txt", `{{index .Tools "golangci-lint"}}`, "docs/adr", false},
		{"parenthesized index", "a.txt", `{{printf "%s" (index .Tools "absent")}}`, "docs/adr", true},
		{"nested values", "a.txt", "{{.Values.ModulePath}}", "docs/adr", false},
		{"root variable", "a.txt", "{{$.ModulePath}}", "docs/adr", false},
		{"range fields", "a.txt", "{{range .Capabilities}}{{.}}{{else}}empty{{end}}", "docs/adr", false},
		{"with pipeline", "a.txt", "{{with .Main}}{{.}}{{end}}", "docs/adr", false},
		{"with dot", "a.txt", "{{with .}}{{.ModulePath}}{{end}}", "docs/adr", false},
		{"with root", "a.txt", "{{with $}}{{.ModulePath}}{{end}}", "docs/adr", false},
		{"with values", "a.txt", "{{with .Values}}{{.ModulePath}}{{end}}", "docs/adr", false},
		{"named template", "a.txt", `{{define "value"}}{{.Main}}{{end}}{{template "value" .}}`, "docs/adr", false},
		{"named missing tool", "a.txt", `{{define "value"}}{{index .Tools "absent"}}{{end}}{{template "value" .}}`, "docs/adr", true},
		{"no template argument", "a.txt", `{{define "value"}}value{{end}}{{template "value"}}`, "docs/adr", false},
		{"chain node", "a.txt", "{{(.Values).ModulePath}}", "docs/adr", false},
		{"path backslash", `a\file.txt`, "value", "docs/adr", true},
		{"path drive", "C:/file.txt", "value", "docs/adr", true},
		{"path root", ".", "value", "docs/adr", true},
		{"path closing delimiter", "file}}.txt", "value", "docs/adr", true},
		{"destination tool missing", `{{index .Tools "absent"}}.txt`, "value", "docs/adr", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			requireRender(t, tc)
		})
	}
}

func TestRenderRejectsOnlyReferencedEmptyInputs(t *testing.T) {
	t.Parallel()
	for _, field := range []string{"ModulePath", "GoVersion", "KitVersion", "Language", "Decisions", "Main"} {
		t.Run(field, func(t *testing.T) {
			t.Parallel()
			requireRenderInputs(t, "{{."+field+"}}", assets.Values{}, true)
		})
	}
	cases := []struct {
		name, body string
		values     assets.Values
		fail       bool
	}{
		{"unused empty", "literal bytes\r\n", assets.Values{}, false},
		{"quoted field", `{{printf "%s" ".ModulePath"}}`, assets.Values{}, false},
		{"comment field", "{{/* .ModulePath */}}value", assets.Values{}, false},
		{"embedded empty", "{{.Values.ModulePath}}", assets.Values{}, true},
		{"grouped empty", "{{(.Values).ModulePath}}", assets.Values{}, true},
		{"grouped indexed tool", `{{index (.Tools) "absent"}}`, assets.Values{}, true},
		{"piped indexed tool", `{{"absent" | index .Tools}}`, assets.Values{}, true},
		{"scoped tool name", "{{with .Tools}}{{.Language}}{{end}}", assets.Values{Tools: map[string]string{"Language": "value"}}, false},
		{"root empty", "{{$.ModulePath}}", assets.Values{}, true},
		{"with dot empty", "{{with .}}{{.ModulePath}}{{end}}", assets.Values{}, true},
		{"with root empty", "{{with $}}{{.ModulePath}}{{end}}", assets.Values{}, true},
		{"empty tool", "{{.Tools.gitleaks}}", assets.Values{Tools: map[string]string{"gitleaks": ""}}, true},
		{"empty indexed tool", `{{index .Tools "gitleaks"}}`, assets.Values{Tools: map[string]string{"gitleaks": ""}}, true},
		{"nested conditional", "{{if has .Capabilities \"go\"}}{{.ModulePath}}{{end}}", assets.Values{}, true},
		{"default backend", "{{.CIBackend}}", assets.Values{}, false},
		{"regular index", "{{index .Capabilities 0}}", assets.Values{Capabilities: []string{"go"}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			requireRenderInputs(t, tc.body, tc.values, tc.fail)
		})
	}
}

func requireRenderInputs(t *testing.T, body string, values assets.Values, fail bool) {
	t.Helper()
	const source = "templates/values.tmpl"
	entry := assets.Template{Source: source, Destination: "result.txt", Ownership: assets.OwnedByKit}
	tree := fstest.MapFS{source: {Data: []byte(body)}}
	got, err := assets.Render(tree, entry, values)
	if fail {
		if !errors.Is(err, assets.ErrRender) || !strings.Contains(err.Error(), entry.Source) ||
			!strings.Contains(err.Error(), entry.Destination) {
			t.Fatalf("expected contextual render error, got %v", err)
		}
		return
	}
	if err != nil || got.SHA256 != install.Digest(got.Data) || got.Template.Source != source {
		t.Fatalf("render = %+v, %v", got, err)
	}
	if !strings.Contains(body, "{{") && string(got.Data) != body {
		t.Fatalf("literal bytes changed: %q", got.Data)
	}
}

func TestRenderNamesUnreadableSource(t *testing.T) {
	t.Parallel()
	entry := assets.Template{Source: "templates/missing.tmpl", Destination: "record.txt"}
	_, err := assets.Render(fstest.MapFS{}, entry, templateValues())
	if !errors.Is(err, assets.ErrRender) || !strings.Contains(err.Error(), entry.Source) ||
		!strings.Contains(err.Error(), entry.Destination) {
		t.Fatalf("read failure = %v", err)
	}
}

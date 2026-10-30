package surface_test

import (
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/surface"
	"github.com/cristianargotti/golaw/internal/model"
)

// view is a page whose script, opened on line 4, holds n lines.
func view(n int, attrs string) string {
	return "<!doctype html>\n<html lang=\"en\">\n<body>\n<script" + attrs + ">\n" + lines(n, "step();") + "</script>\n</body>\n</html>\n"
}

func TestInlineScriptSizeFindsTheLongScripts(t *testing.T) {
	t.Parallel()
	runCases(t, surface.InlineScriptSize(), []fileCase{
		{"an inline script of 41 lines", "views/panel.html", view(41, ""), []string{"views/panel.html:4 inline-script-size"}},
		{"a module script", "views/panel.html", view(41, " type=\"module\""), []string{"views/panel.html:4 inline-script-size"}},
		{"a spliced script counts the same", "views/summary.html", "<script>\n" + lines(20, "step();") + "const id = {{.ID}};\n" + lines(20, "step();") + "</script>\n", []string{"views/summary.html:1 inline-script-size"}},
		{"an htm page", "views/host.htm", view(41, ""), []string{"views/host.htm:4 inline-script-size"}},
	})
}

func TestInlineScriptSizeLeavesTheNearMissesAlone(t *testing.T) {
	t.Parallel()
	runCases(t, surface.InlineScriptSize(), []fileCase{
		{"exactly 40 lines", "views/panel.html", view(40, ""), nil},
		{"blank lines around the body do not count", "views/panel.html", "<script>\n\n\n" + lines(40, "step();") + "\n\n</script>\n", nil},
		{"a script with a source", "views/panel.html", "<script type=\"module\" src=\"/views/panel.js\"></script>\n", nil},
		{"a json island", "views/panel.html", "<script type=\"application/json\">\n" + lines(60, "1,") + "</script>\n", nil},
		{"a style block", "views/panel.html", "<style>\n" + lines(60, "a { color: red; }") + "</style>\n", nil},
		{"a script inside a comment", "views/panel.html", "<!--\n<script>\n" + lines(60, "x();") + "</script>\n-->\n", nil},
		{"two scripts of 40 lines", "views/panel.html", "<script>\n" + lines(40, "a();") + "</script>\n<script>\n" + lines(40, "b();") + "</script>\n", nil},
		{"a script file", "views/panel.js", lines(60, "step();"), nil},
	})
}

func TestInlineScriptSizeNamesTheFileToMoveItTo(t *testing.T) {
	t.Parallel()
	page := lines(11, "<p>x</p>") + "<script>\n" + lines(129, "step();") + "</script>\n"
	res := scanFiles(t, surface.InlineScriptSize(), map[string]string{"views/panel.html": page})
	want := []model.Finding{{
		Path: "views/panel.html", Line: 12, Code: "inline-script-size", Severity: model.SeverityDeny,
		Message: "holds an inline script of 129 lines, the limit is 40: move it to views/panel.js, embed it with go:embed and splice it when the view is served, so the linters read it.",
	}}
	if !slices.EqualFunc(res.Findings, want, sameFinding) {
		t.Fatalf("findings = %+v, want %+v", res.Findings, want)
	}
}

func TestInlineScriptSizeDeniesOnlyAScriptThatCrossesTheLimit(t *testing.T) {
	t.Parallel()
	c := surface.InlineScriptSize()
	expectFound(t, run(t, c, writeInput(c, map[string]string{"views/a.html": view(60, "")}, map[string]string{"views/a.html": view(61, "")})))
	expectFound(t, run(t, c, writeInput(c, map[string]string{"views/a.html": view(10, "")}, map[string]string{"views/a.html": view(41, "")})),
		"views/a.html:4 inline-script-size")
}

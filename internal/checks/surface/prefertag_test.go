package surface_test

import (
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/surface"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

func TestHTMLPreferTagFindsTheRolesOfNativeElements(t *testing.T) {
	t.Parallel()
	runCases(t, surface.HTMLPreferTag(), []fileCase{
		{"role status in markup", "views/panel.html", "<body>\n<div role=\"status\" id=\"state\"></div>\n", []string{"views/panel.html:2 prefer-tag"}},
		{"role img on a span", "views/panel.html", "<span role=\"img\" aria-label=\"up\"></span>\n", []string{"views/panel.html:1 prefer-tag"}},
		{"role button on a div", "views/panel.html", "<div role=\"button\" tabindex=\"0\">go</div>\n", []string{"views/panel.html:1 prefer-tag"}},
		{"upper case markup", "views/panel.html", "<DIV ROLE='status'></DIV>\n", []string{"views/panel.html:1 prefer-tag"}},
		{"an unquoted role", "views/panel.html", "<div role=status></div>\n", []string{"views/panel.html:1 prefer-tag"}},
		{"role status in jsx", "views/panel.jsx", "export const P = ({ n }) => <div role=\"status\">{n}</div>;\n", []string{"views/panel.jsx:1 prefer-tag"}},
		{"role navigation in tsx", "src/menu.tsx", "export const M = () => (\n  <section role=\"navigation\" />\n);\n", []string{"src/menu.tsx:2 prefer-tag"}},
		{"role img in jsx", "views/brand.jsx", "export const B = () => <span role=\"img\">*</span>;\n", []string{"views/brand.jsx:1 prefer-tag"}},
	})
}

func TestHTMLPreferTagLeavesTheNearMissesAlone(t *testing.T) {
	t.Parallel()
	runCases(t, surface.HTMLPreferTag(), []fileCase{
		{"the native output", "views/panel.html", "<output id=\"state\"></output>\n", nil},
		{"role img on an svg", "views/panel.html", "<svg role=\"img\"><title>up</title></svg>\n", nil},
		{"role img on an img", "views/panel.html", "<img role=\"img\" src=\"a.png\" alt=\"up\">\n", nil},
		{"a jsx component", "views/panel.jsx", "export const P = () => <Status role=\"status\" />;\n", nil},
		{"a role inside a comment", "views/panel.html", "<!-- <div role=\"status\"> -->\n", nil},
		{"a role inside a script", "views/panel.html", "<script>\nconst s = '<div role=\"status\">';\n</script>\n", nil},
		{"a role with no native element", "views/panel.html", "<div role=\"tablist\"></div>\n<div role=\"presentation\"></div>\n", nil},
		{"a data attribute", "views/panel.html", "<div data-role=\"status\"></div>\n", nil},
		{"an input with its own role", "views/panel.html", "<input type=\"checkbox\" role=\"checkbox\">\n", nil},
		{"a role a template splices", "views/panel.html", "<div role=\"{{.Role}}\"></div>\n", nil},
		{"a role from an expression", "views/panel.jsx", "export const P = ({ kind }) => <div role={kind} />;\n", nil},
		{"the corrected hub view", "views/panel.jsx", "export const P = ({ rows }) => <output>{rows.length} rows</output>;\n", nil},
	})
}

func TestHTMLPreferTagNamesTheElementAndFailsOnlyUnderStrict(t *testing.T) {
	t.Parallel()
	c := surface.HTMLPreferTag()
	files := map[string]string{"views/panel.html": "<div role=\"status\"></div>\n", "views/panel.jsx": "export const P = () => <div role=\"status\" />;\n"}
	want := []model.Finding{
		{Path: "views/panel.html", Line: 1, Code: "prefer-tag", Message: "uses role=\"status\" on <div> where the native element <output> exists (Sonar Web:S6819): use <output> instead.", Sonar: "Web:S6819", Severity: model.SeverityNote},
		{Path: "views/panel.jsx", Line: 1, Code: "prefer-tag", Message: "uses role=\"status\" on <div> where the native element <output> exists (Sonar javascript:S6819): use <output> instead.", Sonar: "javascript:S6819", Severity: model.SeverityNote},
	}
	if res := scanFiles(t, c, files); !slices.EqualFunc(res.Findings, want, sameFinding) {
		t.Fatalf("parity scan = %+v, want %+v", res.Findings, want)
	}
	in := scanInput(c, files)
	policy := *proof.Policy()
	policy.Surface.Rules = model.SurfaceStrict
	in.Policy = &policy
	for _, f := range run(t, c, in).Findings {
		if f.Severity != model.SeverityFail {
			t.Errorf("strict scan %s severity = %s, want fail", f.Path, f.Severity)
		}
	}
	for _, f := range run(t, c, postInput(c, files)).Findings {
		if f.Severity != model.SeverityNote {
			t.Errorf("hook %s severity = %s, want note", f.Path, f.Severity)
		}
	}
}

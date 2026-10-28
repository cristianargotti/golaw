package surface_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/surface"
)

// region is what a test expects of one region: the body as text and the
// fields Regions computes.
type region struct {
	kind               surface.RegionKind
	body               string
	startLine, endLine int
	lines              int
	spliced, module    bool
}

func TestRegionsFindTheInlineScriptsAndStyles(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		content string
		want    []region
	}{
		"a style and a spliced script": {
			content: "<!doctype html>\n<style>\ntable { x: 1; }\n</style>\n<script>\nlet a = 1;\nlet b = {{.Value}};\n</script>\n",
			want: []region{
				{kind: surface.RegionStyle, body: "table { x: 1; }\n", startLine: 3, endLine: 3, lines: 1},
				{kind: surface.RegionScript, body: "let a = 1;\nlet b = {{.Value}};\n", startLine: 6, endLine: 7, lines: 2, spliced: true},
			},
		},
		"a marker comment before the script": {
			content: "<!-- CHARTS_SCRIPT --><script>\n'use strict';\n/* COMMON_SCRIPT */\nfunction resize(){}\n</script>\n",
			want:    []region{{kind: surface.RegionScript, body: "'use strict';\n/* COMMON_SCRIPT */\nfunction resize(){}\n", startLine: 2, endLine: 4, lines: 3}},
		},
		"an upper case module": {
			content: "<SCRIPT TYPE=\"MODULE\">\nimport x from \"/x.js\";\n</SCRIPT>\n",
			want:    []region{{kind: surface.RegionScript, body: "import x from \"/x.js\";\n", startLine: 2, endLine: 2, lines: 1, module: true}},
		},
		"an unquoted module type": {
			content: "<script type=module>\nrun();\n</script>",
			want:    []region{{kind: surface.RegionScript, body: "run();\n", startLine: 2, endLine: 2, lines: 1, module: true}},
		},
		"an attribute without a value": {
			content: "<script async defer>\nrun();\n</script>",
			want:    []region{{kind: surface.RegionScript, body: "run();\n", startLine: 2, endLine: 2, lines: 1}},
		},
		"a > inside a quoted attribute": {
			content: "<script data-x=\"a>b\">\nrun();\n</script>",
			want:    []region{{kind: surface.RegionScript, body: "run();\n", startLine: 2, endLine: 2, lines: 1}},
		},
		"an indented closing tag": {
			content: "<script>\n  const s = \"</div>\";\n  </script>\n",
			want:    []region{{kind: surface.RegionScript, body: "  const s = \"</div>\";\n", startLine: 2, endLine: 2, lines: 1}},
		},
		"a closing tag with a space": {
			content: "<script>\nrun();\n</script >",
			want:    []region{{kind: surface.RegionScript, body: "run();\n", startLine: 2, endLine: 2, lines: 1}},
		},
		"CRLF line breaks": {
			content: "<script>\r\nrun();\r\n</script>\r\n",
			want:    []region{{kind: surface.RegionScript, body: "run();\r\n", startLine: 2, endLine: 2, lines: 1}},
		},
		"a one line script": {
			content: "<p><script>run()</script></p>",
			want:    []region{{kind: surface.RegionScript, body: "run()", startLine: 1, endLine: 1, lines: 1}},
		},
		"an empty script": {
			content: "<script></script>",
			want:    []region{{kind: surface.RegionScript, startLine: 1, endLine: 1}},
		},
		"an unclosed script runs to the end": {
			content: "<script>\nrun();\n",
			want:    []region{{kind: surface.RegionScript, body: "run();\n", startLine: 2, endLine: 2, lines: 1}},
		},
		"a tag cut at the end of the file": {
			content: "<p>x</p><script",
			want:    []region{{kind: surface.RegionScript, startLine: 1, endLine: 1}},
		},
		"a style with a media attribute": {
			content: "<style media=\"print\">\nbody { color: black; }\n</style>",
			want:    []region{{kind: surface.RegionStyle, body: "body { color: black; }\n", startLine: 2, endLine: 2, lines: 1}},
		},
		"an unclosed style": {
			content: "<style>\nb {}\n",
			want:    []region{{kind: surface.RegionStyle, body: "b {}\n", startLine: 2, endLine: 2, lines: 1}},
		},
		"a style tag a string spells is inside the script (near)": {
			content: "<script>\nconst t = \"<style>\";\n</script>\n",
			want:    []region{{kind: surface.RegionScript, body: "const t = \"<style>\";\n", startLine: 2, endLine: 2, lines: 1}},
		},
		"a served script (near)":          {content: "<script src=\"/views/panel.js\"></script>"},
		"a served module (near)":          {content: "<script type=\"module\" src=\"/views/panel.js\"></script>"},
		"a JSON block (near)":             {content: "<script type=\"application/json\">{\"a\":1}</script>"},
		"a type with parameters (near)":   {content: "<script type=\"text/javascript; charset=utf-8\">run()</script>"},
		"a commented script (near)":       {content: "<!-- <script>eval(\"1\")</script> -->\n<p>x</p>\n"},
		"an unclosed comment (near)":      {content: "<!-- <script>run()</script>\n"},
		"names that only start so (near)": {content: "<scripts>run()</scripts><styled>x</styled>"},
		"no markup at all (near)":         {content: "plain text\n"},

		// A view the Go side serves keeps client markup in template blocks.
		"a text/template block (near)":   {content: "<script type=\"text/template\" id=\"row\">\n<tr><td>{{.Name}}</td></tr>\n</script>\n"},
		"a text/x-template block (near)": {content: "<script type=\"text/x-template\">\n<p>{{ count }}</p>\n</script>\n"},
		"an import map (near)":           {content: "<script type=\"importmap\">{\"imports\": {\"a\": \"/a.js\"}}</script>"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			content := []byte(tc.content)
			got := surface.Regions(content)
			if len(got) != len(tc.want) {
				t.Fatalf("Regions = %+v, want %d regions", got, len(tc.want))
			}
			for i, r := range got {
				seen := region{
					kind: r.Kind, body: string(content[r.Start:r.End]), startLine: r.StartLine, endLine: r.EndLine,
					lines: r.Lines(), spliced: r.Spliced, module: r.Module,
				}
				if seen != tc.want[i] {
					t.Errorf("region %d = %+v, want %+v", i, seen, tc.want[i])
				}
			}
		})
	}
}

func TestABrokenOpeningTagAnswersAnEmptyBody(t *testing.T) {
	t.Parallel()
	got := surface.Regions([]byte("<script data-x=\"open>\nrun();\n"))
	if len(got) != 1 || got[0].Lines() != 0 || got[0].Kind != surface.RegionScript {
		t.Fatalf("Regions = %+v, want one empty script region", got)
	}
}

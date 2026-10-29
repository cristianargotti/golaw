package surface

import (
	"slices"
	"testing"
)

// shape renders tokens as kind:text, comments and jsx text included.
func shape(tokens []token) []string {
	out := make([]string, 0, len(tokens))
	for _, t := range tokens {
		out = append(out, t.kind.String()+":"+t.text)
	}
	return out
}

func TestLexJSTellsTheLiterals(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		src  string
		jsx  bool
		want []string
	}{
		{"a division and a regular expression", "a = b / c;\nx = /re[/]x/g.test(y);", false, []string{
			"ident:a", "punct:=", "ident:b", "punct:/", "ident:c", "punct:;",
			"ident:x", "punct:=", "regex:/re[/]x/g", "punct:.", "ident:test", "punct:(", "ident:y", "punct:)", "punct:;",
		}},
		{"a regular expression after a keyword", "return /a/.test(s);", false, []string{
			"ident:return", "regex:/a/", "punct:.", "ident:test", "punct:(", "ident:s", "punct:)", "punct:;",
		}},
		{"a template and its substitutions", "t = `a${b + `c${d}`}e`;", false, []string{
			"ident:t", "punct:=", "template:`a${b + `c${d}`}e`", "ident:b", "punct:+", "template:`c${d}`", "ident:d", "punct:;",
		}},
		{"strings and comments", "s = 'it\\'s'; // eval(x)\n/* eval(y) */ q = \"a\";", false, []string{
			"ident:s", "punct:=", "string:'it\\'s'", "punct:;", "comment:// eval(x)", "comment:/* eval(y) */", "ident:q", "punct:=", "string:\"a\"", "punct:;",
		}},
		{"operators stay whole", "a === b && c?.d => e == f;", false, []string{
			"ident:a", "punct:===", "ident:b", "punct:&&", "ident:c", "punct:?.", "ident:d", "punct:=>", "ident:e", "punct:==", "ident:f", "punct:;",
		}},
		{"an unterminated string ends at the line", "s = \"open\nt = 1;", false, []string{
			"ident:s", "punct:=", "string:\"open", "ident:t", "punct:=", "number:1", "punct:;",
		}},
		{"an unterminated comment runs to the end", "a; /* never closed\neval(x)", false, []string{
			"ident:a", "punct:;", "comment:/* never closed\neval(x)",
		}},
		{"a jsx element", "const a = <p class=\"x\">eval(y) {z}</p>;", true, []string{
			"ident:const", "ident:a", "punct:=", "tag:p", "ident:class", "punct:=", "string:\"x\"", "text:eval(y) ", "punct:{", "ident:z", "punct:}", "punct:;",
		}},
		{"nested and self-closing jsx", "x = <A b={1}><c/></A>;", true, []string{
			"ident:x", "punct:=", "tag:A", "ident:b", "punct:=", "punct:{", "number:1", "punct:}", "tag:c", "punct:;",
		}},
		{"a generic arrow is no element", "const id = <T,>(x: T) => x;", true, []string{
			"ident:const", "ident:id", "punct:=", "punct:<", "ident:T", "punct:,", "punct:>", "punct:(", "ident:x", "punct::", "ident:T", "punct:)", "punct:=>", "ident:x", "punct:;",
		}},
		{"a comparison in a jsx file", "if (a < b) {}", true, []string{
			"ident:if", "punct:(", "ident:a", "punct:<", "ident:b", "punct:)", "punct:{", "punct:}",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := shape(lexJS([]byte(tt.src), 1, tt.jsx)); !slices.Equal(got, tt.want) {
				t.Fatalf("tokens = %q\nwant %q", got, tt.want)
			}
		})
	}
}

func TestLexJSCountsLinesFromTheUnit(t *testing.T) {
	t.Parallel()
	tokens := lexJS([]byte("a\n\n`x\ny`\nb"), 5, false)
	lines := make([]int, 0, len(tokens))
	for _, tk := range tokens {
		lines = append(lines, tk.line)
	}
	if !slices.Equal(lines, []int{5, 7, 9}) {
		t.Fatalf("lines = %v, want 5, 7 and 9", lines)
	}
}

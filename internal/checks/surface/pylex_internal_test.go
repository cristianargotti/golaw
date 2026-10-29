package surface

import (
	"slices"
	"testing"
)

func pyShape(tokens []pyToken) []string {
	out := make([]string, 0, len(tokens))
	for _, t := range tokens {
		out = append(out, t.kind.String()+":"+t.text)
	}
	return out
}

func TestLexPythonTellsTheLiterals(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{"prefixes and triple quotes", "x = rb'a' + f\"b{c}\" + \"\"\"d\ne\"\"\"", []string{
			"ident:x", "punct:=", "string:rb'a'", "punct:+", "string:f\"b{c}\"", "punct:+", "string:\"\"\"d\ne\"\"\"",
		}},
		{"a hash inside a string", "s = \"a # b\"  # eval(x)", []string{
			"ident:s", "punct:=", "string:\"a # b\"", "comment:# eval(x)",
		}},
		{"operators stay whole", "a == b != c := d ** e -> f", []string{
			"ident:a", "punct:==", "ident:b", "punct:!=", "ident:c", "punct::=", "ident:d", "punct:**", "ident:e", "punct:->", "ident:f",
		}},
		{"an unterminated string ends at the line", "s = 'open\nt = 1", []string{
			"ident:s", "punct:=", "string:'open", "ident:t", "punct:=", "number:1",
		}},
		{"an unterminated triple string runs to the end", "s = \"\"\"open\neval(x)", []string{
			"ident:s", "punct:=", "string:\"\"\"open\neval(x)",
		}},
		{"a raw string keeps its backslashes", "p = r\"a\\\"b\"", []string{
			"ident:p", "punct:=", "string:r\"a\\\"b\"",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := pyShape(lexPython([]byte(tt.src))); !slices.Equal(got, tt.want) {
				t.Fatalf("tokens = %q\nwant %q", got, tt.want)
			}
		})
	}
}

func TestLexPythonPlacesEveryToken(t *testing.T) {
	t.Parallel()
	tokens := lexPython([]byte("def f():\n    return \"\"\"a\nb\"\"\"\n"))
	type place struct{ line, endLine, col int }
	got := make([]place, 0, len(tokens))
	for _, tk := range tokens {
		got = append(got, place{tk.line, tk.endLine, tk.col})
	}
	want := []place{{1, 1, 0}, {1, 1, 4}, {1, 1, 5}, {1, 1, 6}, {1, 1, 7}, {2, 2, 4}, {2, 3, 11}}
	if !slices.Equal(got, want) {
		t.Fatalf("places = %v, want %v", got, want)
	}
}

func TestPyStringValueRefusesARunTimeFString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		text  string
		value string
		ok    bool
	}{
		{`"abc"`, "abc", true},
		{`b'abc'`, "abc", true},
		{`"""abc"""`, "abc", true},
		{`f"plain"`, "plain", true},
		{`f"{x}-y"`, "", false},
		{`r"a\b"`, `a\b`, true},
	}
	for _, tt := range tests {
		value, ok := pyStringValue(pyToken{kind: tokString, text: tt.text})
		if value != tt.value || ok != tt.ok {
			t.Errorf("%s = %q, %t, want %q, %t", tt.text, value, ok, tt.value, tt.ok)
		}
	}
}

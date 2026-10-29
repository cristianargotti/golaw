package surface

import (
	"slices"
	"testing"
)

func TestLexJSReadsTheEdgesOfAUnit(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{"a hashbang", "#!/usr/bin/env node\nx", []string{"comment:#!/usr/bin/env node", "ident:x"}},
		{"html-like comments", "a <!-- b\n --> c\nd", []string{"ident:a", "comment:<!-- b", "comment:--> c", "ident:d"}},
		{"a decrement before a greater sign", "a-->b", []string{"ident:a", "punct:--", "punct:>", "ident:b"}},
		{"a line continuation in a string", "s = 'a\\\nb'; t", []string{"ident:s", "punct:=", "string:'a\\\nb'", "punct:;", "ident:t"}},
		{"an unterminated template", "`a${b", []string{"template:`a${b", "ident:b"}},
		{"an unterminated regular expression", "x = /ab\ny", []string{"ident:x", "punct:=", "regex:/ab", "ident:y"}},
	}
	for _, tt := range tests {
		if got := shape(lexJS([]byte(tt.src), 1, false)); !slices.Equal(got, tt.want) {
			t.Errorf("%s: tokens = %q\nwant %q", tt.name, got, tt.want)
		}
	}
}

func TestLexJSIsTotalOverEveryPrefix(t *testing.T) {
	t.Parallel()
	src := "#!/x\nconst a = <A b=\"c\" {...d} e='f'>t {`g${h /* i */ + 'j'}`}<k/></A>;\n" +
		"x = /[/\\]]+/g; // l\nif (a < b) { y = `m\\`${{n: 1}.n}` }\n<!-- o\n--> p\n"
	for i := range len(src) + 1 {
		for _, jsx := range []bool{false, true} {
			_ = lexJS([]byte(src[:i]), 1, jsx)
		}
	}
	if got := stringValue(token{kind: tokString, text: `'it\'s`}); got != "it's" {
		t.Fatalf("value = %q, want the unterminated text unescaped", got)
	}
	if got := stringValue(token{kind: tokString}); got != "" {
		t.Fatalf("value = %q, want empty", got)
	}
	if got := tokenKind(42).String(); got != "unknown" {
		t.Fatalf("kind = %q, want unknown", got)
	}
}

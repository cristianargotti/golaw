package surface

import (
	"slices"
	"testing"
)

func TestLexPythonJoinsLinesAndKeepsColumns(t *testing.T) {
	t.Parallel()
	tokens := lexPython([]byte("x = 1 + \\\n  2\ny = \\\r\n3"))
	if got, want := pyShape(tokens), []string{"ident:x", "punct:=", "number:1", "punct:+", "number:2", "ident:y", "punct:=", "number:3"}; !slices.Equal(got, want) {
		t.Fatalf("tokens = %q, want %q", got, want)
	}
	if two := tokens[4]; two.line != 2 || two.col != 2 {
		t.Fatalf("2 at line %d column %d, want line 2 column 2", two.line, two.col)
	}
	if odd := pyShape(lexPython([]byte("a \\ b"))); !slices.Equal(odd, []string{"ident:a", "punct:\\", "ident:b"}) {
		t.Fatalf("a stray backslash = %q, want an operator", odd)
	}
}

func TestLexPythonIsTotalOverEveryPrefix(t *testing.T) {
	t.Parallel()
	src := "#!/usr/bin/env python3\nx = rb'a\\'' + f\"{y!r}\" # c\ns = '''t\n\\'''' \\\nz = .5e3 ** 2\n"
	for i := range len(src) + 1 {
		_ = lexPython([]byte(src[:i]))
	}
	if value, ok := pyStringValue(pyToken{kind: tokString, text: `f"{{x}}"`}); !ok || value != "{{x}}" {
		t.Fatalf("value = %q, %t, want the doubled braces as text", value, ok)
	}
	if value, ok := pyStringValue(pyToken{kind: tokString, text: `"a\"b"`}); !ok || value != `a"b` {
		t.Fatalf("value = %q, %t, want the escaped quote resolved", value, ok)
	}
}

func TestShadowedReadsEveryBinding(t *testing.T) {
	t.Parallel()
	src := "from a import (b, eval as exec)\nimport os.path, c as d\n"
	shadow := shadowed(pySignificant(lexPython([]byte(src))))
	for _, name := range []string{"b", "exec", "os", "d"} {
		if !shadow[name] {
			t.Errorf("%s is not shadowed in %v", name, shadow)
		}
	}
	if shadow["eval"] {
		t.Errorf("eval imported under another name stays the builtin: %v", shadow)
	}
}

func TestSplitTokensStopsWhenAsked(t *testing.T) {
	t.Parallel()
	tokens := lexJS([]byte("a, (b, c), d"), 1, false)
	var entries int
	for range splitTokens(tokens) {
		entries++
		break
	}
	if entries != 1 {
		t.Fatalf("entries = %d, want the first alone", entries)
	}
}

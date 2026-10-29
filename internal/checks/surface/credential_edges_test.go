package surface_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/surface"
)

// TestHardcodedCredentialReadsTheEdgesOfTheScriptForms pins the shapes
// around the name rules: a template without a substitution is a literal,
// an attribute after an expression container still belongs to its element,
// an import source and a statement after an element are no attribute.
func TestHardcodedCredentialReadsTheEdgesOfTheScriptForms(t *testing.T) {
	t.Parallel()
	runCases(t, surface.HardcodedCredential(), []fileCase{
		{"a template without a substitution", "a.js", "const " + word + " = `" + value + "`;\n", []string{"a.js:1 hardcoded-credential"}},
		{"an attribute after a container", "a.jsx", "export const F = () => <Login onDone={() => { go(); }} " + word + "=\"" + value + "\" />;\n", []string{"a.jsx:1 hardcoded-credential"}},
		{"a statement after an element", "a.tsx", "const f = <A />;\nconst " + word + " = \"" + value + "\";\n", []string{"a.tsx:2 hardcoded-credential"}},
		{"a call on the literal", "a.js", "const " + word + " = \"" + value + "\".trim();\n", nil},
		{"an import source", "a.js", "import cfg from \"./DB_" + upper + "=" + value + "\";\n", nil},
		{"an object key alone", "a.js", "const c = { \"DB_" + upper + "=" + value + "\": 1 };\n", nil},
		{"a number key", "a.js", "const c = { 1: \"" + value + "\" };\n", nil},
		{"an equals without a name", "a.js", "const env = \"=" + value + "\";\n", nil},
		{"a pair inside a word", "a.js", "const env = \"x-DB_" + upper + "=" + value + "\";\n", nil},
		{"a second URL with the password", "a.py", "dsn = \"see http://docs/a then postgres://admin:" + value + "@db/app\"\n", []string{"a.py:1 hardcoded-credential"}},
		{"a value inside brackets", "a.py", "rows = [" + word + " == \"" + value + "\"]\n", nil},
		{"an annotation with a dotted type", "a.py", word + ": typing.Final = \"" + value + "\"\n", []string{"a.py:1 hardcoded-credential"}},
		{"a quoted ENV value with a blank", "Dockerfile", "FROM scratch\nENV MODE=\"a b\" DB_" + upper + "='" + value + "'\n", []string{"Dockerfile:2 hardcoded-credential"}},
		{"an empty ENV", "Dockerfile", "FROM scratch\nENV\n", nil},
	})
}

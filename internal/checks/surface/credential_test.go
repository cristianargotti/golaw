package surface_test

import (
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/surface"
	"github.com/cristianargotti/golaw/internal/model"
)

// The credential word in three spellings and a literal value.
const (
	word    = "pass" + "word"
	capital = "Pass" + "word"
	upper   = "PASS" + "WORD"
	value   = "zq81" + "-kept"
)

// credFix is the tail of the S2068 messages (M6.6.5).
const credFix = " (Sonar S2068): generate it at run time (secrets.token_urlsafe(16), crypto.randomUUID()) or read it from the environment."

func TestHardcodedCredentialFindsTheLiterals(t *testing.T) {
	t.Parallel()
	runCases(t, surface.HardcodedCredential(), []fileCase{
		{"a const assigned", "app.js", "const " + word + " = \"" + value + "\";\n", []string{"app.js:1 hardcoded-credential"}},
		{"a typed let", "a.ts", "let db" + capital + ": string = \"" + value + "\";\n", []string{"a.ts:1 hardcoded-credential"}},
		{"an object value", "a.js", "const c = { " + word + ": \"" + value + "\" };\n", []string{"a.js:1 hardcoded-credential"}},
		{"a quoted key", "a.js", "const c = { \"passwd\": \"" + value + "\" };\n", []string{"a.js:1 hardcoded-credential"}},
		{"a member assigned", "a.js", "config.pwd = '" + value + "';\n", []string{"a.js:1 hardcoded-credential"}},
		{"NAME=value in a string", "a.js", "const env = \"DB_" + upper + "=" + value + "\";\n", []string{"a.js:1 hardcoded-credential"}},
		{"a URL with a password", "a.js", "const dsn = \"postgres://admin:" + value + "@db.internal/app\";\n", []string{"a.js:1 hardcoded-credential"}},
		{"an html script", "views/a.html", "<script>\nconst " + word + " = \"" + value + "\";\n</script>\n", []string{"views/a.html:2 hardcoded-credential"}},
		{"a jsx attribute", "a.jsx", "export const F = () => <Login " + word + "=\"" + value + "\" />;\n", []string{"a.jsx:1 hardcoded-credential"}},
		{"a python assignment", "test/deploy/check_install.py", "\n\n" + word + " = \"" + value + "\"\n", []string{"test/deploy/check_install.py:3 hardcoded-credential"}},
		{"an annotated assignment", "a.py", word + ": str = \"" + value + "\"\n", []string{"a.py:1 hardcoded-credential"}},
		{"a keyword argument", "a.py", "login(user=\"admin\", " + word + "=\"" + value + "\")\n", []string{"a.py:1 hardcoded-credential"}},
		{"a dict value", "a.py", "creds = {\"passphrase\": \"" + value + "\"}\n", []string{"a.py:1 hardcoded-credential"}},
		{"NAME=value inside a python string", "a.py", "env = \"POSTGRES_" + upper + "=" + value + "\"\n", []string{"a.py:1 hardcoded-credential"}},
		{"a URL in python", "a.py", "dsn = \"postgres://admin:" + value + "@db/app\"\n", []string{"a.py:1 hardcoded-credential"}},
		{"an ENV default", "Dockerfile", "FROM scratch\nENV DB_" + upper + "=" + value + "\n", []string{"Dockerfile:2 hardcoded-credential"}},
		{"an ARG default", "Dockerfile", "FROM scratch\nARG ADMIN_" + upper + "=\"" + value + "\"\n", []string{"Dockerfile:2 hardcoded-credential"}},
		{"the legacy ENV form", "deploy/app.Dockerfile", "FROM scratch\nENV " + upper + " " + value + "\n", []string{"deploy/app.Dockerfile:2 hardcoded-credential"}},
		{"ENV with two pairs", "Dockerfile", "FROM scratch\nENV MODE=prod DB_" + upper + "=" + value + "\n", []string{"Dockerfile:2 hardcoded-credential"}},
	})
}

func TestHardcodedCredentialLeavesTheNearMissesAlone(t *testing.T) {
	t.Parallel()
	runCases(t, surface.HardcodedCredential(), []fileCase{
		{"an empty literal", "a.js", "const " + word + " = \"\";\n", nil},
		{"the name as its value", "a.js", "const " + word + " = \"" + word + "\";\n", nil},
		{"a label of the name", "a.js", "const db" + capital + " = \"DB " + capital + ":\";\n", nil},
		{"a pt-BR label", "a.js", "const labels = { " + word + ": \"Senha\" };\n", nil},
		{"a translated label with a colon", "a.jsx", "labels." + word + " = \"Contraseña:\";\n", nil},
		{"a validation message", "a.js", "errors." + word + " = \"A senha é obrigatória\";\n", nil},
		{"a python label map", "a.py", "labels = {\"" + word + "\": \"campo obrigatório\"}\n", nil},
		{"an english sentence", "a.ts", "const hint: string = \"\";\nhints." + word + " = \"Use at least twelve characters\";\n", nil},
		{"stars", "a.py", word + " = \"******\"\n", nil},
		{"x marks", "a.py", word + " = \"xxxx\"\n", nil},
		{"a dollar placeholder", "a.js", "const " + word + " = \"${DB_" + upper + "}\";\n", nil},
		{"a template placeholder", "a.py", word + " = \"{{ .Value }}\"\n", nil},
		{"a printf placeholder", "a.py", word + " = \"%s\"\n", nil},
		{"an angle placeholder", "a.py", word + " = \"<" + word + ">\"\n", nil},
		{"an environment read", "a.py", word + " = os.environ[\"DB_" + upper + "\"]\n", nil},
		{"process.env", "a.js", "const " + word + " = process.env.DB_" + upper + ";\n", nil},
		{"a template built at run time", "a.js", "const " + word + " = `${prefix}-x`;\n", nil},
		{"an f-string built at run time", "a.py", word + " = f\"{prefix}-x\"\n", nil},
		{"a concatenation", "a.py", word + " = \"ab\" + suffix\n", nil},
		{"the corrected hub test", "test/deploy/test_install.py", "import secrets\n\nsecret = secrets.token_urlsafe(16)\n", nil},
		{"secret, token and api_key are not denied", "a.py", "secret = \"" + value + "\"\ntoken = \"" + value + "\"\napi_key = \"" + value + "\"\n", nil},
		{"the word is not the last one", "a.py", word + "_hash = \"" + value + "\"\n", nil},
		{"a field name", "a.js", "const " + word + "Field = \"" + value + "\";\n", nil},
		{"a comparison", "a.py", "if " + word + " == \"" + value + "\":\n    pass\n", nil},
		{"a ternary branch", "a.js", "const v = ok ? " + word + " : \"" + value + "\";\n", nil},
		{"a URL without a password", "a.py", "dsn = \"postgres://admin@db.internal/app\"\n", nil},
		{"a URL with a placeholder", "a.py", "dsn = \"postgres://admin:${PW}@db/app\"\n", nil},
		{"an ARG without a default", "Dockerfile", "FROM scratch\nARG DB_" + upper + "\n", nil},
		{"an ENV from an ARG", "Dockerfile", "FROM scratch\nARG DB_" + upper + "\nENV DB_" + upper + "=$DB_" + upper + "\n", nil},
		{"a Dockerfile comment", "Dockerfile", "FROM scratch\n# ENV DB_" + upper + "=" + value + "\n", nil},
		{"a word in a comment", "a.js", "// " + word + " = \"" + value + "\"\n", nil},
		{"a fixture", "pkg/testdata/a.py", word + " = \"" + value + "\"\n", nil},
	})
}

func TestHardcodedCredentialNamesTheFormAndTheFix(t *testing.T) {
	t.Parallel()
	res := scanFiles(t, surface.HardcodedCredential(), map[string]string{
		"Dockerfile": "FROM scratch\nARG DB_" + upper + "=" + value + "\n",
		"a.py":       word + " = \"" + value + "\"\nlogin(" + word + "=\"" + value + "\")\ndsn = \"postgres://admin:" + value + "@db/app\"\n",
		"b.js":       "const c = { " + word + ": \"" + value + "\" };\nconst e = \"DB_" + upper + "=" + value + "\";\n",
	})
	want := []model.Finding{
		{Path: "Dockerfile", Line: 2, Code: "hardcoded-credential", Message: "sets a literal default for DB_" + upper + " (Sonar S6472): pass it as a build secret or at run time from the environment.", Sonar: "docker:S6472", Severity: model.SeverityDeny},
		{Path: "a.py", Line: 1, Code: "hardcoded-credential", Message: "assigns a literal to " + word + credFix, Sonar: "python:S2068", Severity: model.SeverityDeny},
		{Path: "a.py", Line: 2, Code: "hardcoded-credential", Message: "passes a literal to " + word + credFix, Sonar: "python:S2068", Severity: model.SeverityDeny},
		{Path: "a.py", Line: 3, Code: "hardcoded-credential", Message: "holds a password in the user info of a URL" + credFix, Sonar: "python:S2068", Severity: model.SeverityDeny},
		{Path: "b.js", Line: 1, Code: "hardcoded-credential", Message: "binds a literal to " + word + credFix, Sonar: "javascript:S2068", Severity: model.SeverityDeny},
		{Path: "b.js", Line: 2, Code: "hardcoded-credential", Message: "holds a literal value for DB_" + upper + " inside a string" + credFix, Sonar: "javascript:S2068", Severity: model.SeverityDeny},
	}
	if !slices.EqualFunc(res.Findings, want, sameFinding) {
		t.Fatalf("findings = %+v\nwant %+v", res.Findings, want)
	}
}

func TestHardcodedCredentialDeniesOnlyANewLiteral(t *testing.T) {
	t.Parallel()
	c := surface.HardcodedCredential()
	legacy := map[string]string{"a.py": word + " = \"" + value + "\"\n"}
	expectFound(t, run(t, c, writeInput(c, legacy, map[string]string{"a.py": "import os\n\n" + word + " = \"" + value + "\"\n"})))
	expectFound(t, run(t, c, writeInput(c, legacy, map[string]string{"a.py": word + " = \"" + value + "\"\nadmin_" + word + " = \"" + value + "\"\n"})),
		"a.py:2 hardcoded-credential")
}

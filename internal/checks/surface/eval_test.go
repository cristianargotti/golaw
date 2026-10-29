package surface_test

import (
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/surface"
	"github.com/cristianargotti/golaw/internal/model"
)

// evalFix is the tail of every surface.code-eval message (M6.6.5).
const evalFix = " (Sonar S1523): export the code from a module and call it; writing it to a file and requiring it is the same execution."

func TestCodeEvalFindsTextRunAsCode(t *testing.T) {
	t.Parallel()
	runCases(t, surface.CodeEval(), []fileCase{
		{"a global eval", "app.js", "const x = 1;\neval(code);\n", []string{"app.js:2 code-eval"}},
		{"new Function in a test", "test/view/host.test.cjs", "const render = new Function(\"view\", \"return view.title;\");\n", []string{"test/view/host.test.cjs:1 code-eval"}},
		{"Function as a global", "lib.mjs", "const f = Function(\"a\", \"return a\");\n", []string{"lib.mjs:1 code-eval"}},
		{"setTimeout with a string", "a.js", "setTimeout(\"tick()\", 10);\n", []string{"a.js:1 code-eval"}},
		{"setInterval with a template", "a.js", "setInterval(`tick()`, 10);\n", []string{"a.js:1 code-eval"}},
		{"globalThis.setTimeout with a string", "a.ts", "globalThis.setTimeout(\"tick()\", 10);\n", []string{"a.ts:1 code-eval"}},
		{"vm required under node:", "host.test.cjs", "const vm = require(\"node:vm\");\nvm.runInThisContext(\"1 + 1\");\n", []string{"host.test.cjs:2 code-eval"}},
		{"vm destructured and renamed", "a.cjs", "const { runInNewContext: run } = require(\"vm\");\nrun(\"1\");\n", []string{"a.cjs:2 code-eval"}},
		{"vm imported as a namespace", "a.mjs", "import * as vm from \"vm\";\nnew vm.Script(\"1\");\n", []string{"a.mjs:2 code-eval"}},
		{"vm default import", "b.mjs", "import vm from \"node:vm\";\nnew vm.SourceTextModule(\"x\");\n", []string{"b.mjs:2 code-eval"}},
		{"vm named import", "a.ts", "import { compileFunction } from \"node:vm\";\ncompileFunction(\"return 1\");\n", []string{"a.ts:2 code-eval"}},
		{"vm required inline", "c.cjs", "require(\"vm\").runInContext(\"1\", context);\n", []string{"c.cjs:1 code-eval"}},
		{"eval in a template substitution", "a.js", "const s = `${eval(x)}`;\n", []string{"a.js:1 code-eval"}},
		{"eval inside an html script", "views/a.html", "<html>\n<body>\n<script>\neval(code);\n</script>\n</body>\n</html>\n", []string{"views/a.html:4 code-eval"}},
		{"a spliced html script", "views/b.html", "<script>\nconst id = {{.ID}};\neval(id);\n</script>\n", []string{"views/b.html:3 code-eval"}},
		{"eval in a tsx expression", "a.tsx", "export const A = () => <div onClick={() => eval(x)}>go</div>;\n", []string{"a.tsx:1 code-eval"}},
		{"the lexer recovers after a generic arrow", "b.tsx", "const id = <T,>(x: T) => x;\neval(code);\n", []string{"b.tsx:2 code-eval"}},
		{"a node script without extension", "bin/run", "#!/usr/bin/env node\neval(process.argv[2]);\n", []string{"bin/run:2 code-eval"}},
	})
}

func TestCodeEvalLeavesTheNearMissesAlone(t *testing.T) {
	t.Parallel()
	runCases(t, surface.CodeEval(), []fileCase{
		{"eval in a line comment", "a.js", "// eval(code)\n", nil},
		{"eval in a block comment", "a.js", "/* eval(code) */\n", nil},
		{"eval in a string", "a.js", "const s = \"eval(code)\";\n", nil},
		{"eval in a regular expression", "a.js", "const re = /eval\\(x\\)/;\n", nil},
		{"a member named eval", "a.js", "engine.eval(code);\n", nil},
		{"an optional member named eval", "a.js", "engine?.eval(code);\n", nil},
		{"a method named eval", "a.ts", "class Engine {\n  eval(code: string): number {\n    return 1;\n  }\n}\n", nil},
		{"an object method named eval", "a.js", "const o = { eval(code) { return code; } };\n", nil},
		{"a function named eval", "a.js", "function eval(code) {}\n", nil},
		{"setTimeout with a function", "a.js", "setTimeout(() => tick(), 10);\n", nil},
		{"setTimeout of another object", "a.js", "timers.setTimeout(\"x\", 1);\n", nil},
		{"a vm that is not the module", "a.cjs", "const vm = require(\"./vm-helper\");\nvm.runInThisContext(\"1\");\n", nil},
		{"eval in jsx text", "a.jsx", "export const A = () => <p>never call eval(x) here</p>;\n", nil},
		{"an html script with a source", "views/c.html", "<script src=\"/app.js\">eval(x)</script>\n", nil},
		{"an html template script", "views/d.html", "<script type=\"text/template\">eval(x)</script>\n", nil},
		{"a script inside an html comment", "views/e.html", "<!-- <script>eval(x)</script> -->\n", nil},
		{"html markup text", "views/f.html", "<p>eval(x)</p>\n", nil},
		{"a minified bundle", "views/app.min.js", "eval(x);\n", nil},
		{"a fixture", "internal/x/testdata/a.js", "eval(x);\n", nil},
		{"a generated mock", "test/mocks/a.js", "eval(x);\n", nil},
		{"a declaration file", "types.d.ts", "declare function eval(x: string): any;\neval(y);\n", nil},
		{"a go file", "main.go", "package main\n\n// eval(x)\n", nil},
		{"the corrected hub test", "test/view/host.test.cjs", "const assert = require(\"node:assert\");\nconst { render } = require(\"../../views/render.cjs\");\n\nassert.strictEqual(render({ title: \"panel\" }), \"panel\");\n", nil},
	})
}

func TestCodeEvalNamesTheConstructAndTheLanguage(t *testing.T) {
	t.Parallel()
	res := scanFiles(t, surface.CodeEval(), map[string]string{
		"test/view/host.test.cjs": "const render = new Function(\"view\", \"return view.title;\");\n",
		"src/run.ts":              "const vm = require(\"node:vm\");\nvm.runInThisContext(code);\n",
	})
	want := []model.Finding{
		{Path: "src/run.ts", Line: 2, Code: "code-eval", Message: "runs text as code with vm.runInThisContext" + evalFix, Sonar: "typescript:S1523", Severity: model.SeverityDeny},
		{Path: "test/view/host.test.cjs", Line: 1, Code: "code-eval", Message: "runs text as code with new Function" + evalFix, Sonar: "javascript:S1523", Severity: model.SeverityDeny},
	}
	if !slices.EqualFunc(res.Findings, want, sameFinding) {
		t.Fatalf("findings = %+v, want %+v", res.Findings, want)
	}
}

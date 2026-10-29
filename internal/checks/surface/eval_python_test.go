package surface_test

import (
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/surface"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestCodeEvalFindsThePythonBuiltins(t *testing.T) {
	t.Parallel()
	runCases(t, surface.CodeEval(), []fileCase{
		{"the eval builtin", "scripts/load.py", "value = eval(text)\n", []string{"scripts/load.py:1 code-eval"}},
		{"the exec builtin", "scripts/load.py", "exec(code, scope)\n", []string{"scripts/load.py:1 code-eval"}},
		{"a space before the parenthesis", "a.py", "x = eval (text)\n", []string{"a.py:1 code-eval"}},
		{"inside a function", "a.py", "def run(text):\n    return eval(text)\n", []string{"a.py:2 code-eval"}},
		{"a test", "test/test_load.py", "def test_load():\n    assert exec(\"x = 1\") is None\n", []string{"test/test_load.py:2 code-eval"}},
		{"a python script without extension", "bin/tool", "#!/usr/bin/env python3\nexec(open(name).read())\n", []string{"bin/tool:2 code-eval"}},
	})
}

func TestCodeEvalLeavesThePythonNearMissesAlone(t *testing.T) {
	t.Parallel()
	runCases(t, surface.CodeEval(), []fileCase{
		{"a method named eval", "a.py", "model.eval()\n", nil},
		{"a method named exec", "a.py", "conn.exec(query)\n", nil},
		{"a local function named eval", "a.py", "def eval(text):\n    return text\n\n\nprint(eval(\"x\"))\n", nil},
		{"an imported eval", "a.py", "from evaluator import eval\n\neval(\"x\")\n", nil},
		{"an imported exec under another name", "a.py", "from runner import run as exec\n\nexec(\"x\")\n", nil},
		{"an assigned exec", "a.py", "exec = runner.exec\nexec(\"x\")\n", nil},
		{"literal_eval", "a.py", "import ast\n\nvalue = ast.literal_eval(text)\n", nil},
		{"eval in a comment", "a.py", "# eval(x)\n", nil},
		{"eval in a string", "a.py", "doc = \"eval(x)\"\n", nil},
		{"eval in a docstring", "a.py", "\"\"\"Never eval(x).\"\"\"\n", nil},
		{"a keyword argument named eval", "a.py", "run(eval=True)\n", nil},
		{"a method defined as eval", "a.py", "class Model:\n    def eval(self):\n        return 1\n", nil},
		{"a fixture", "pkg/testdata/a.py", "eval(x)\n", nil},
	})
}

func TestCodeEvalNamesThePythonConstruct(t *testing.T) {
	t.Parallel()
	res := scanFiles(t, surface.CodeEval(), map[string]string{"scripts/load.py": "\nexec(code)\n"})
	want := []model.Finding{{Path: "scripts/load.py", Line: 2, Code: "code-eval", Message: "runs text as code with exec" + evalFix, Sonar: "python:S1523", Severity: model.SeverityDeny}}
	if !slices.EqualFunc(res.Findings, want, sameFinding) {
		t.Fatalf("findings = %+v, want %+v", res.Findings, want)
	}
}

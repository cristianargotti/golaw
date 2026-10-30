package surface_test

import (
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/surface"
	"github.com/cristianargotti/golaw/internal/model"
)

// def is a function whose body holds n counted lines.
func def(name string, n int) string { return "def " + name + "():\n" + lines(n, "    total = 1") }

func TestPythonFunctionLengthCountsTheBody(t *testing.T) {
	t.Parallel()
	runCases(t, surface.PythonFunctionLength(), []fileCase{
		{"a body of 41 lines", "tools/report.py", "\n" + def("build", 41), []string{"tools/report.py:2 python-function-length"}},
		{"a method of 41 lines", "tools/report.py", "class Report:\n    def build(self):\n" + lines(41, "        total = 1"), []string{"tools/report.py:2 python-function-length"}},
		{"a signature over several lines", "a.py", "def build(\n    a,\n    b,\n):\n" + lines(41, "    a = b"), []string{"a.py:1 python-function-length"}},
		{"an async def", "a.py", "async def build():\n" + lines(41, "    await x"), []string{"a.py:1 python-function-length"}},
		{"blank lines inside count", "a.py", "def build():\n" + lines(20, "    x = 1") + "\n" + lines(20, "    x = 2"), []string{"a.py:1 python-function-length"}},
		{"a nested def counts in both", "a.py", "def outer():\n    def inner():\n" + lines(41, "        x = 1") + "    return inner\n", []string{"a.py:1 python-function-length", "a.py:2 python-function-length"}},
		{"a script without extension", "bin/report", "#!/usr/bin/env python3\n" + def("build", 41), []string{"bin/report:2 python-function-length"}},
	})
}

func TestPythonFunctionLengthLeavesTheNearMissesAlone(t *testing.T) {
	t.Parallel()
	runCases(t, surface.PythonFunctionLength(), []fileCase{
		{"exactly 40 lines", "a.py", def("build", 40), nil},
		{"comments do not count", "a.py", "def build():\n" + lines(20, "    x = 1") + lines(5, "    # why") + lines(20, "    x = 1"), nil},
		{"a comment at column zero stays inside", "a.py", "def build():\n" + lines(20, "    x = 1") + "# note\n" + lines(20, "    x = 1"), nil},
		{"trailing blank lines do not count", "a.py", def("build", 40) + "\n\n\nprint(1)\n", nil},
		{"a one line body", "a.py", "def f(): return 1\n", nil},
		{"a test function", "test/test_report.py", "def test_build():\n" + lines(60, "    assert x"), nil},
		{"a test module named after its subject", "tests/report_test.py", "def test_build():\n" + lines(60, "    assert x"), nil},
		{"a fixture", "pkg/testdata/a.py", def("build", 60), nil},
	})
}

func TestPythonFunctionLengthIsFeedback(t *testing.T) {
	t.Parallel()
	c := surface.PythonFunctionLength()
	files := map[string]string{"a.py": def("build", 41)}
	want := []model.Finding{{Path: "a.py", Line: 1, Code: "python-function-length", Message: "function build is 41 lines, the limit is 40: give it one job.", Severity: model.SeverityFail}}
	if res := scanFiles(t, c, files); !slices.EqualFunc(res.Findings, want, sameFinding) {
		t.Fatalf("scan = %+v, want %+v", res.Findings, want)
	}
	if res := run(t, c, postInput(c, files)); len(res.Findings) != 1 || res.Findings[0].Severity != model.SeverityNote {
		t.Fatalf("post = %+v, want one note", res.Findings)
	}
	if res := run(t, c, writeInput(c, nil, files)); len(res.Findings) != 1 || res.Findings[0].Severity != model.SeverityNote {
		t.Fatalf("pre write = %+v, want one note that never denies", res.Findings)
	}
}

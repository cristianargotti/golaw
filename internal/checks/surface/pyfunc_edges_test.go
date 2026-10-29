package surface_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/surface"
)

// TestPythonFunctionLengthReadsTheEdgesOfABody pins the boundaries the
// indentation reader meets: a continuation inside brackets at column zero
// stays in the body, a docstring over several lines counts each line, a
// conftest is a test file, a def inside a call is no definition and a
// header that never ends is no body.
func TestPythonFunctionLengthReadsTheEdgesOfABody(t *testing.T) {
	t.Parallel()
	runCases(t, surface.PythonFunctionLength(), []fileCase{
		{"a continuation at column zero", "a.py", "def render():\n    x = call(\n1)\n" + lines(39, "    y = 2"), []string{"a.py:1 python-function-length"}},
		{"a docstring over several lines", "a.py", "def render():\n    \"\"\"one\n    two\n    \"\"\"\n" + lines(38, "    y = 2"), []string{"a.py:1 python-function-length"}},
		{"the next def ends the body", "a.py", def("render", 40) + def("draw", 41), []string{"a.py:42 python-function-length"}},
		{"a conftest", "tests/conftest.py", def("render", 60), nil},
		{"a def word inside a call", "a.py", "run(def_ = 1)\nx = [\ndef\n]\n", nil},
		{"a header that never ends", "a.py", "def render(\n" + lines(60, "    a,"), nil},
	})
}

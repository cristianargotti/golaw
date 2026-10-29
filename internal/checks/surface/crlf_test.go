package surface_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/surface"
)

// crlf is the text as a Windows checkout with core.autocrlf writes it.
func crlf(text string) string { return strings.ReplaceAll(text, "\n", "\r\n") }

// TestTheLengthLawsCountACheckoutWithCRLF holds the counts to the line
// breaks alone: a carriage return before each one changes no count, so a
// file or a body at the limit never fires on Windows.
func TestTheLengthLawsCountACheckoutWithCRLF(t *testing.T) {
	t.Parallel()
	runCases(t, surface.FileLength(), []fileCase{
		{"300 lines with CRLF", "scripts/smoke.sh", crlf("#!/bin/sh\n" + lines(299, "echo step")), nil},
		{"301 lines with CRLF", "scripts/smoke.sh", crlf("#!/bin/sh\n" + lines(300, "echo step")), []string{"scripts/smoke.sh:0 file-length"}},
	})
	runCases(t, surface.PythonFunctionLength(), []fileCase{
		{"a body of 40 lines with CRLF", "a.py", crlf(def("build", 40)), nil},
		{"blank and comment lines with CRLF", "a.py", crlf("def build():\n" + lines(20, "    x = 1") + "\n" + lines(5, "    # why") + lines(19, "    x = 1")), nil},
		{"a body of 41 lines with CRLF", "a.py", crlf(def("build", 41)), []string{"a.py:1 python-function-length"}},
	})
}

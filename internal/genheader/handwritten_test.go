package genheader_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/genheader"
)

func TestHandwrittenReadsTheMarkerOfTheLeadingComments(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		content string
		line    int
		reason  string
		marked  bool
	}{
		"red: a marker with its reason":  {"// golaw:handwritten the vendor API streams a chunked body\npackage vendor\n", 1, "the vendor API streams a chunked body", true},
		"red: a bare marker":             {"// golaw:handwritten\npackage vendor\n", 1, "", true},
		"red: a bare marker with blanks": {"//golaw:handwritten   \npackage vendor\n", 1, "", true},
		"red: after the package doc":     {"// Package vendor streams.\n//\n// golaw:handwritten the API streams\npackage vendor\n", 3, "the API streams", true},
		"near: after the package clause": {"package vendor\n\n// golaw:handwritten the API streams\n", 0, "", false},
		"near: not the first word":       {"// see golaw:handwritten for the escape\npackage vendor\n", 0, "", false},
		"near: a longer word":            {"// golaw:handwrittenx reason\npackage vendor\n", 0, "", false},
		"near: no comment at all":        {"package vendor\n", 0, "", false},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			line, reason, marked := genheader.HandwrittenAt([]byte(tc.content))
			if line != tc.line || reason != tc.reason || marked != tc.marked {
				t.Errorf("HandwrittenAt = %d, %q, %v, want %d, %q, %v", line, reason, marked, tc.line, tc.reason, tc.marked)
			}
			if reason2, marked2 := genheader.Handwritten([]byte(tc.content)); reason2 != tc.reason || marked2 != tc.marked {
				t.Errorf("Handwritten = %q, %v", reason2, marked2)
			}
		})
	}
}

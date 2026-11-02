package genheader_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/genheader"
)

func TestSameIgnoringVersionIgnoresOnlyTheVersionToken(t *testing.T) {
	t.Parallel()
	line := func(version, kind string, template int) string {
		return genheader.Header{Version: version, Kind: kind, Template: template}.Line() + "\n\npackage p\n"
	}
	tests := map[string]struct {
		a, b string
		want bool
	}{
		"another kit version":        {line("0.2.0", "port", 1), line("0.3.0", "port", 1), true},
		"equal bytes without header": {"package p\n", "package p\n", true},
		"a hand edited body":         {line("0.3.0", "port", 1), line("0.3.0", "port", 1) + "// edit\n", false},
		"another kind":               {line("0.3.0", "port", 1), line("0.3.0", "usecase", 1), false},
		"another template":           {line("0.3.0", "port", 1), line("0.3.0", "port", 2), false},
		"a stripped header":          {line("0.3.0", "port", 1), "package p\n", false},
		"different bytes without":    {"package p\n", "package q\n", false},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := genheader.SameIgnoringVersion([]byte(tc.a), []byte(tc.b)); got != tc.want {
				t.Errorf("SameIgnoringVersion = %v, want %v", got, tc.want)
			}
		})
	}
}

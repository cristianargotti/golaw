package main

import (
	"bytes"
	"strings"
	"testing"
)

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, ErrAcceptance
}

func TestReport(t *testing.T) {
	t.Parallel()
	cases := []struct {
		rows []row
		want bool
	}{
		{[]row{measured("protocol", "allow", nil)}, true},
		{[]row{measured("protocol", "deny", ErrAcceptance)}, false},
		{[]row{{"live", "timeout", statusSkip, "record absent", true}}, false},
		{[]row{{"path", "alias", statusSkip, "case sensitive", false}}, true},
		{[]row{{"protocol", "invalid", "UNKNOWN", "", false}}, false},
	}
	for _, tc := range cases {
		var out bytes.Buffer
		if report(&out, tc.rows, "2.1.299", "digest") != tc.want {
			t.Fatal(out.String())
		}
		if !strings.Contains(out.String(), "CATEGORY\tAGENT\tOS\tVERSION\tBINARY\tSTATUS") {
			t.Fatal(out.String())
		}
	}
	var out bytes.Buffer
	if !report(&out, []row{{"path", "alias", statusSkip, "case\nsensitive", false}}, "version", "digest") ||
		!strings.Contains(out.String(), "\tSKIP\t") ||
		strings.Contains(out.String(), "case\nsensitive") {
		t.Fatal(out.String())
	}
	if report(failingWriter{}, nil, "", "") {
		t.Fatal("failed writer passed")
	}
}

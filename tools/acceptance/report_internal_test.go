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

// TestReportNamesTheRequiredRows marks every row required or optional and
// closes a failed run with the required rows that did not pass, so a
// release run that stops on a missing live record says which rows stopped
// it (section 10.3 item 6).
func TestReportNamesTheRequiredRows(t *testing.T) {
	t.Parallel()
	rows := []row{
		measured("protocol", "allow", nil),
		{"live", "host-timeout", statusSkip, "record absent", true},
		{"vendor", "plugin-test", statusSkip, "not applicable", false},
		measured("protocol", "deny", ErrAcceptance),
	}
	var out bytes.Buffer
	if report(&out, rows, "2.1.299", "digest") {
		t.Fatal(out.String())
	}
	text := out.String()
	for _, want := range []string{
		"\tSTATUS\tREQUIRED\tCASE\t",
		"\tPASS\trequired\tallow\t", "\tSKIP\toptional\tplugin-test\t",
		"rows that fail the run: live/host-timeout SKIP required, protocol/deny FAIL\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in %q", want, text)
		}
	}
	out.Reset()
	if !report(&out, rows[:1], "2.1.299", "digest") || strings.Contains(out.String(), "fail the run") {
		t.Fatal(out.String())
	}
}

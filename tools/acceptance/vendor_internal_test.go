package main

import (
	"context"
	"strings"
	"testing"
)

func TestVersionFloor(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		text string
		ok   bool
	}{
		{"2.1.294 (Claude Code)", false},
		{"2.1.295 (Claude Code)", true},
		{"2.1.299 (Claude Code)", true},
		{"2.2.0", true},
		{"3.0.0", true},
		{"1.99.999", false},
		{"2.0.999", false},
		{"unknown", false},
		{"999999999999999999999999.1.1", false},
	} {
		if err := versionFloor(tc.text); (err == nil) != tc.ok {
			t.Fatalf("%q: %v", tc.text, err)
		}
	}
}

func TestVendorRows(t *testing.T) {
	t.Parallel()
	s := testSuite(t)
	rows, version := s.vendorRows(t.Context(), true)
	if version != "2.1.299 (Claude Code)" || len(rows) != 4 {
		t.Fatalf("%q %+v", version, rows)
	}
	assertPass(t, rows[0])
	assertPass(t, rows[1])
	if rows[2].status != statusSkip || rows[2].required {
		t.Fatalf("the plugin test row must be an optional skip: %+v", rows[2])
	}
	assertPass(t, rows[3])
	s.lookup = func(string) (string, error) { return "", ErrAcceptance }
	rows, version = s.vendorRows(t.Context(), true)
	if version != "absent" || len(rows) != 2 || rows[0].status != statusSkip || !rows[0].required {
		t.Fatalf("%q %+v", version, rows)
	}
	s.lookup = func(name string) (string, error) { return name, nil }
	s.execute = func(context.Context, command) result { return result{stdout: "2.1.294"} }
	rows, version = s.vendorRows(t.Context(), true)
	if len(rows) != 1 || rows[0].status != statusFail || version != "2.1.294" {
		t.Fatalf("%q %+v", version, rows)
	}
	s.execute = func(_ context.Context, c command) result {
		if c.args[0] == "--version" {
			return result{stdout: "2.1.295"}
		}
		return result{code: 1, stderr: "validator failed"}
	}
	rows, _ = s.vendorRows(t.Context(), false)
	if len(rows) != 4 || rows[1].status != statusFail || rows[2].status != statusSkip || rows[3].status != statusSkip {
		t.Fatalf("%+v", rows)
	}
	s.execute = func(context.Context, command) result { return result{err: ErrAcceptance} }
	rows, _ = s.vendorRows(t.Context(), true)
	if len(rows) != 1 || !strings.Contains(rows[0].detail, "version") {
		t.Fatalf("%+v", rows)
	}
}

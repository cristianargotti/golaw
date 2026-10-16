package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stream is a govulncheck JSON stream with one dependency-only finding and
// two findings reached from the code, one of them without a fix.
const stream = `{"config": {"scanner_name": "govulncheck"}}
{"progress": {"message": "Scanning"}}
{"osv": {"id": "GO-2026-0001"}}
{"finding": {"osv": "GO-2026-0001", "fixed_version": "v1.27.2", "trace": [{"module": "stdlib", "version": "v1.27.0"}]}}
{"finding": {"osv": "GO-2026-0002", "fixed_version": "v1.27.2", "trace": [{"module": "stdlib", "package": "net/http", "function": "Serve"}, {"module": "example.test/m", "package": "example.test/m", "function": "main"}]}}
{"finding": {"osv": "GO-2026-0003", "trace": [{"module": "example.test/dep", "package": "example.test/dep", "function": "Do"}]}}
`

// waiversFile writes a policy file with the given waivers and returns its path.
func waiversFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "golaw.yml")
	if err := os.WriteFile(path, []byte("golaw: 1\nlanguage: pt-BR\n"+body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadReachableKeepsOnlyFunctionLevelFindings(t *testing.T) {
	t.Parallel()
	got, err := readReachable(strings.NewReader(stream))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got["GO-2026-0002"] != "v1.27.2" || got["GO-2026-0003"] != "" {
		t.Fatalf("reachable = %v", got)
	}
	if _, err := readReachable(strings.NewReader("{")); err == nil {
		t.Fatal("a truncated stream must fail")
	}
}

// executeCase is one run of the tool over the stream with a waivers file.
type executeCase struct {
	waivers string
	args    []string
	code    int
	want    []string
}

// executeCases lists the waiver outcomes: applied, missing, expired, and
// the inputs the tool refuses.
func executeCases() map[string]executeCase {
	waived := "vulnerability_waivers:\n  - { id: GO-2026-0002, until: 2026-12-31, record: docs/adr/0003-serve.md }\n  - { id: GO-2026-0003, until: 2027-01-31, record: docs/adr/0004-dep.md }\n"
	expired := "vulnerability_waivers:\n  - { id: GO-2026-0002, until: 2026-10-01, record: docs/adr/0003-serve.md }\n"
	today := []string{"-today", "2026-10-14"}
	return map[string]executeCase{
		"all waived": {waived, today, exitClean, []string{
			"vulngate: GO-2026-0002 waived until 2026-12-31 (docs/adr/0003-serve.md)",
			"vulngate: GO-2026-0003 waived until 2027-01-31 (docs/adr/0004-dep.md)",
		}},
		"reachable without waiver": {"", today, exitReachable, []string{
			"vulngate: GO-2026-0002 reachable, fixed in v1.27.2",
			"vulngate: GO-2026-0003 reachable, fixed in no version yet",
		}},
		"expired waiver": {expired, today, exitReachable, []string{
			"vulngate: GO-2026-0002 reachable, fixed in v1.27.2",
			"vulngate: GO-2026-0003 reachable, fixed in no version yet",
			"vulngate: waiver GO-2026-0002 expired on 2026-10-01, remove it",
		}},
		"no date":    {waived, nil, exitFailure, nil},
		"bad waiver": {"vulnerability_waivers:\n  - { id: GO-1, until: soon, record: r }\n", today, exitFailure, nil},
		"no record":  {"vulnerability_waivers:\n  - { id: GO-1, until: 2026-12-31 }\n", today, exitFailure, nil},
		"bad flag":   {waived, []string{"-nope"}, exitFailure, nil},
		"not yaml":   {"::\n  - [", today, exitFailure, nil},
	}
}

func TestExecuteAppliesWaiversAndExpiry(t *testing.T) {
	t.Parallel()
	for name, tc := range executeCases() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			args := append([]string{"-waivers", waiversFile(t, tc.waivers)}, tc.args...)
			var stdout, stderr bytes.Buffer
			if got := execute(args, strings.NewReader(stream), &stdout, &stderr); got != tc.code {
				t.Fatalf("exit = %d, want %d, stdout = %q, stderr = %q", got, tc.code, stdout.String(), stderr.String())
			}
			if tc.want != nil && stdout.String() != strings.Join(tc.want, "\n")+"\n" {
				t.Fatalf("stdout = %q", stdout.String())
			}
		})
	}
}

func TestExecuteWithoutReachableFindingsIsClean(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	args := []string{"-waivers", waiversFile(t, ""), "-today", "2026-10-14"}
	if got := execute(args, strings.NewReader(`{"finding": {"osv": "GO-1", "trace": [{"module": "m"}]}}`), &stdout, &stderr); got != exitClean {
		t.Fatalf("exit = %d, stderr = %q", got, stderr.String())
	}
	if stdout.String() != "vulngate: no reachable vulnerability\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
	missing := []string{"-waivers", filepath.Join(t.TempDir(), "none.yml"), "-today", "2026-10-14"}
	if got := execute(missing, strings.NewReader(""), &stdout, &stderr); got != exitFailure {
		t.Fatalf("missing waivers exit = %d", got)
	}
}

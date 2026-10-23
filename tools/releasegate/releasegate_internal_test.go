package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tagged is the commit every fixture's runs are about.
const tagged = "c0ffee5a17b02e57a7e1e2d0c0ffee5a17b02e57"

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestDecideOverTheRunsOfCI(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		ok   bool
		line string
	}{
		"green.json":       {true, "o commit " + tagged + " tem CI verde na main: execução 41, tentativa 2 de ci.yml por push"},
		"failed.json":      {false, "a última execução de ci.yml por push na main (44, tentativa 1) está completed com conclusão failure"},
		"in-progress.json": {false, "(41, tentativa 2) está in_progress com conclusão nenhuma"},
		"other-sha.json":   {false, "o commit " + tagged + " não pode ser lançado: não há execução de ci.yml por push na main para ele"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ok, line, err := Decide(fixture(t, name), tagged)
			if err != nil || ok != tc.ok || !strings.Contains(line, tc.line) {
				t.Fatalf("Decide = %v, %q, %v, want %v with %q", ok, line, err, tc.ok, tc.line)
			}
		})
	}
}

func TestDecideRefusesWhatIsNotAnAnswerOfTheEndpoint(t *testing.T) {
	t.Parallel()
	for name, runs := range map[string]string{
		"not JSON":  "<html>",
		"no list":   `{"message": "Not Found"}`,
		"null list": `{"workflow_runs": null}`,
		"not runs":  `{"workflow_runs": [1]}`,
	} {
		if ok, _, err := Decide([]byte(runs), tagged); ok || !errors.Is(err, errRuns) {
			t.Errorf("%s: %v, %v", name, ok, err)
		}
	}
	if ok, _, err := Decide([]byte(`{"workflow_runs": []}`), tagged); ok || err != nil {
		t.Errorf("an empty history refuses without an error: %v, %v", ok, err)
	}
}

// failingWriter refuses every write.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errRuns }

func TestExecuteAnswersTheExitCodeTheLaneReads(t *testing.T) {
	t.Parallel()
	green := filepath.Join("testdata", "green.json")
	cases := map[string]struct {
		args   []string
		code   int
		stdout string
		stderr string
	}{
		"releasable":      {[]string{"-sha", tagged, "-runs", green}, exitReleasable, "releasegate: o commit " + tagged + " tem CI verde", ""},
		"refused":         {[]string{"-sha", tagged, "-runs", filepath.Join("testdata", "failed.json")}, exitRefused, "não pode ser lançado", ""},
		"no sha":          {[]string{"-runs", green}, exitFailure, "", `-sha "" is not a full lower case commit id`},
		"a short sha":     {[]string{"-sha", tagged[:12], "-runs", green}, exitFailure, "", "is not a full lower case commit id"},
		"no runs":         {[]string{"-sha", tagged}, exitFailure, "", "-runs names no file"},
		"an absent file":  {[]string{"-sha", tagged, "-runs", filepath.Join(t.TempDir(), "absent.json")}, exitFailure, "", "read the runs of ci.yml"},
		"an unknown flag": {[]string{"-tag", "v1.2.3"}, exitFailure, "", "flag provided but not defined: -tag"},
		"a foreign file":  {[]string{"-sha", tagged, "-runs", filepath.Join("testdata", "..", "main.go")}, exitFailure, "", "read the runs of ci.yml"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			if got := execute(tc.args, &stdout, &stderr); got != tc.code {
				t.Fatalf("exit = %d, stdout = %q, stderr = %q", got, stdout.String(), stderr.String())
			}
			if !strings.Contains(stdout.String(), tc.stdout) || !strings.Contains(stderr.String(), tc.stderr) {
				t.Fatalf("stdout = %q, stderr = %q", stdout.String(), stderr.String())
			}
		})
	}
	if got := execute([]string{"-sha", tagged, "-runs", green}, failingWriter{}, failingWriter{}); got != exitFailure {
		t.Fatalf("a decision nobody can read exits %d", got)
	}
}

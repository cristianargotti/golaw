package main

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/install"
)

func TestDoctorManagedUsesCommittedHashes(t *testing.T) {
	t.Parallel()
	for _, change := range []string{"removed entry", "replaced hash"} {
		t.Run(change, func(t *testing.T) {
			t.Parallel()
			root := committedManagedRepository(t)
			policy := string(renderedPolicy(t))
			if change == "replaced hash" {
				policy = managedDoctorPolicy(t, "edited\n")
			}
			writeFiles(t, root, map[string]string{
				"golaw.yml": policy,
				"golaw.mk":  "edited\n",
			})
			for _, asJSON := range []bool{false, true} {
				requireCommittedManagedDrift(t, root, asJSON)
			}
		})
	}
}

func TestDoctorManagedKeepsFileFailuresNonfatal(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"missing", "unknown"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			root := committedManagedRepository(t)
			path := filepath.Join(root, "golaw.mk")
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			code := "managed-missing"
			if state == "unknown" {
				code = "managed-unreadable"
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			ta := runManagedDoctor(t, root, true)
			lines := managedJSONLines(t, ta.stdout)
			if len(lines) != 1 || lines[0].Key != "golaw.mk" || lines[0].State != state || lines[0].Code != code {
				t.Fatalf("file failure = %+v, want %s %s", lines, state, code)
			}
		})
	}
}

func TestDoctorManagedSkipsUnavailablePolicies(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"outside git", "uncommitted", "invalid committed"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			root := unavailableManagedRepository(t, kind)
			ta := runManagedDoctor(t, root, true)
			if lines := managedJSONLines(t, ta.stdout); len(lines) != 0 {
				t.Fatalf("a policy not in force produced managed lines: %+v", lines)
			}
		})
	}
}

func requireCommittedManagedDrift(t *testing.T, root string, asJSON bool) {
	t.Helper()
	ta := runManagedDoctor(t, root, asJSON)
	if asJSON {
		lines := managedJSONLines(t, ta.stdout)
		if len(lines) != 1 || lines[0].Key != "golaw.mk" || lines[0].State != "drift" || lines[0].Code != "managed-drift" {
			t.Fatalf("committed managed evidence = %+v", lines)
		}
		return
	}
	if count := strings.Count(ta.stdout.String(), "doctor.managed golaw.mk: drift [managed-drift] "); count != 1 ||
		strings.Count(ta.stdout.String(), "doctor.managed ") != 1 {
		t.Fatalf("committed drift appears %d times: %q", count, ta.stdout.String())
	}
}

func runManagedDoctor(t *testing.T, root string, asJSON bool) testApp {
	t.Helper()
	ta, _ := doctorApp(t)
	args := []string{"doctor", root}
	if asJSON {
		args = append(args, "--json")
	}
	if code := ta.run(t.Context(), args); code != exitClean || ta.stderr.Len() != 0 {
		t.Fatalf("json %t: exit = %d, stderr = %q", asJSON, code, ta.stderr.String())
	}
	return ta
}

func unavailableManagedRepository(t *testing.T, kind string) string {
	t.Helper()
	root := t.TempDir()
	if kind != "outside git" {
		root = repository(t, false)
	}
	policy := managedDoctorPolicy(t, "recorded\n")
	if kind == "invalid committed" {
		policy = "golaw: invalid\n"
	}
	writeFiles(t, root, map[string]string{
		"golaw.yml": policy,
		"golaw.mk":  "edited\n",
	})
	if kind == "invalid committed" {
		gitRun(t, root, "add", "-A")
		gitRun(t, root, "commit", "-q", "-m", "build: record an invalid policy")
	}
	return root
}

func committedManagedRepository(t *testing.T) string {
	t.Helper()
	root := repository(t, false)
	writeFiles(t, root, map[string]string{
		"golaw.yml": managedDoctorPolicy(t, "recorded\n"),
		"golaw.mk":  "recorded\n",
	})
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "build: adopt the managed policy")
	return root
}

func managedDoctorPolicy(t *testing.T, content string) string {
	t.Helper()
	managed := "managed:\n  - path: golaw.mk\n    sha256: " + install.Digest([]byte(content)) + "\n"
	return strings.Replace(string(renderedPolicy(t)), "managed: []\n", managed, 1)
}

func managedJSONLines(t *testing.T, out io.Reader) []doctor.Line {
	t.Helper()
	var lines []doctor.Line
	decoder := json.NewDecoder(out)
	for index := 0; ; index++ {
		var report doctor.Report
		if err := decoder.Decode(&report); errors.Is(err, io.EOF) {
			return lines
		} else if err != nil {
			t.Fatal(err)
		}
		if report.Schema != 1 {
			t.Fatalf("doctor schema = %d", report.Schema)
		}
		for _, line := range report.Lines {
			if line.ID != "doctor.managed" {
				continue
			}
			if index != 0 {
				t.Fatalf("managed line belongs to later report %d: %+v", index, line)
			}
			lines = append(lines, line)
		}
	}
}

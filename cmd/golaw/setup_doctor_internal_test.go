package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/gittest"
)

func TestDoctorPrintsSetupOncePerRepository(t *testing.T) {
	t.Parallel()
	for _, asJSON := range []bool{false, true} {
		t.Run(map[bool]string{false: "text", true: "json"}[asJSON], func(t *testing.T) {
			t.Parallel()
			repo := gittest.New(t)
			ta, _ := doctorApp(t)
			args := []string{"doctor", repo.Root}
			if asJSON {
				args = append(args, "--json")
			}
			if code := ta.run(t.Context(), args); code != exitClean {
				t.Fatalf("exit = %d, stderr = %q", code, ta.stderr.String())
			}
			if count := setupReceiptCount(t, ta.stdout, asJSON); count != 1 {
				t.Fatalf("setup receipt appears %d times", count)
			}
		})
	}
}

// setupReceiptCount counts the setup receipt lines doctor printed, read
// from its JSON reports or from its text.
func setupReceiptCount(t *testing.T, out *bytes.Buffer, asJSON bool) int {
	t.Helper()
	if !asJSON {
		return strings.Count(out.String(), "doctor.setup-receipt ")
	}
	count := 0
	decoder := json.NewDecoder(out)
	for {
		var report doctor.Report
		err := decoder.Decode(&report)
		if errors.Is(err, io.EOF) {
			return count
		}
		if err != nil {
			t.Fatal(err)
		}
		count += setupLines(t, report)
	}
}

// setupLines counts the setup receipt lines of one report, each of which
// must say that setup did not arm the repository.
func setupLines(t *testing.T, report doctor.Report) int {
	t.Helper()
	count := 0
	for _, line := range report.Lines {
		if line.ID != "doctor.setup-receipt" {
			continue
		}
		count++
		if line.State != "not-armed" {
			t.Fatalf("optional setup = %+v", line)
		}
	}
	return count
}

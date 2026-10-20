package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"testing"
	"time"
)

//nolint:paralleltest // the measurements time the rungs alone; a test beside them would skew what they publish
func TestM2Measurements(t *testing.T) {
	if os.Getenv("GOLAW_M2_MEASURE") != "1" {
		t.Skip("native measurements require GOLAW_M2_MEASURE=1")
	}
	s := m2RealSuite(t)
	s.m2.Live = os.Getenv("GOLAW_M2_LIVE_DB") == "1"
	metadata, err := s.timingMetadataM2(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	samples := m2Samples(t, s)
	summary, err := summarizeM2(samples)
	if err != nil {
		t.Fatal(err)
	}
	payload := struct {
		OS       string            `json:"os"`
		Arch     string            `json:"arch"`
		Binary   string            `json:"binary"`
		Metadata map[string]string `json:"metadata"`
		Samples  []m2Sample        `json:"samples"`
	}{runtime.GOOS, runtime.GOARCH, s.digest, metadata, samples}
	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	comparison, budgetErr := budgetM2(samples, m2WarmBudget)
	m2Publish(t, fmt.Sprintf("%s\n%s\n%s\n%s\n", summary, comparison, m2Contention(t, s), body))
	if budgetErr != nil {
		t.Fatal(budgetErr)
	}
	t.Log("M2_MEASUREMENTS_COMPLETE")
}

// m2Samples times five samples of each rung with cold and with warm caches.
func m2Samples(t *testing.T, s *suite) []m2Sample {
	t.Helper()
	samples := make([]m2Sample, 0, 30)
	for _, rung := range []string{"pre-commit", "commit-msg", "pre-push"} {
		for _, cache := range []string{"cold", "warm"} {
			samples = append(samples, m2Series(t, s, rung, cache)...)
		}
	}
	return samples
}

// m2Series times the five samples of one rung and cache state; a warm
// series primes its caches with one untimed invocation first.
func m2Series(t *testing.T, s *suite, rung, cache string) []m2Sample {
	t.Helper()
	root := filepath.Join(t.TempDir(), rung, cache)
	series := make([]m2Sample, 0, 5)
	for index := range 5 {
		current := root
		if cache == "cold" {
			current = filepath.Join(root, strconv.Itoa(index))
		}
		c, err := s.timingM2(t.Context(), rung, current)
		if err != nil {
			t.Fatalf("timing preparation: %v", err)
		}
		if cache == "warm" && index == 0 {
			m2MeasuredResult(t, rung, s.execute(t.Context(), c))
		}
		start := time.Now()
		got := s.execute(t.Context(), c)
		elapsed := time.Since(start)
		m2MeasuredResult(t, rung, got)
		series = append(series, m2Sample{
			Rung: rung, Cache: cache,
			Milliseconds: elapsed.Milliseconds(), Code: got.code,
		})
	}
	return series
}

// m2Publish logs the measurements and appends them to the job summary of
// a native CI run.
func m2Publish(t *testing.T, text string) {
	t.Helper()
	t.Log(text)
	path := os.Getenv("GITHUB_STEP_SUMMARY")
	if path == "" {
		return
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := file.WriteString(text)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		t.Fatalf("publish measurements: %v %v", writeErr, closeErr)
	}
}

func m2RealSuite(t *testing.T) *suite {
	t.Helper()
	s := newSuite(testRoot(t), t.TempDir(), runM2Command, exec.LookPath)
	if err := s.prepareM2(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := s.lifecycleM2(t.Context(), "setup-clean"); err != nil {
		t.Fatal(err)
	}
	return s
}

func m2MeasuredResult(t *testing.T, rung string, got result) {
	t.Helper()
	if reason := m2MeasuredReason(rung, got); reason != "" {
		t.Fatalf("%s did not complete measured: %s: %s", rung, reason, got.stdout+got.stderr)
	}
}

// m2MeasuredReason is why the answer of a rung is no measured completion,
// or empty when it is one. It reads the statuses the producers print: the
// verdict line of a report, which always counts its unmeasured checks, the
// line of a check left unmeasured, the push line of a scanner that could
// not judge, and the line of files over the read bound.
func m2MeasuredReason(rung string, got result) string {
	if got.err != nil {
		return got.err.Error()
	}
	text := got.stdout + got.stderr
	verdicts := regexp.MustCompile(`(?m)^verdict \w+: .*, (\d+) unmeasured$`)
	for _, match := range verdicts.FindAllStringSubmatch(text, -1) {
		if match[1] != "0" {
			return "the verdict counts " + match[1] + " unmeasured checks"
		}
	}
	statuses := regexp.MustCompile(`(?m)^\S+ unmeasured |govulncheck could not judge it|over the read bound, not judged here`)
	if status := statuses.FindString(text); status != "" {
		return "unmeasured: " + status
	}
	advisory := regexp.MustCompile(`GO-\d{4}-\d{4,}`)
	if got.code == 0 || rung == "pre-push" && got.code > 0 && advisory.MatchString(text) {
		return ""
	}
	return "exit " + strconv.Itoa(got.code)
}

func TestM2MeasuredStatuses(t *testing.T) {
	t.Parallel()
	clean := "verdict clean: 0 findings (0 enforced, 0 notes), 32 excluded, 0 unmeasured\n"
	tests := []struct {
		name     string
		rung     string
		got      result
		measured bool
	}{
		{"zero unmeasured", "pre-commit", result{stdout: clean}, true},
		{"positive unmeasured", "pre-commit", result{
			stdout: "verdict clean: 0 findings (0 enforced, 0 notes), 31 excluded, 1 unmeasured\n",
		}, false},
		{"named unmeasured check", "pre-commit", result{
			stdout: "goast.file-length unmeasured the file is over the read bound\n" + clean,
		}, false},
		{"scanner failure", "pre-push", result{
			stderr: "golaw pre-push: main: govulncheck could not judge it, the pipeline judges it: exit status 2\n",
		}, false},
		{"over the read bound", "pre-commit", result{
			stdout: clean + "golaw pre-commit: over the read bound, not judged here: big.bin; the pipeline judges them\n",
		}, false},
		{"measured advisory", "pre-push", result{code: 1, stderr: "golaw pre-push: main: GO-2026-0001 is reachable\n"}, true},
		{"advisory outside the push", "pre-commit", result{code: 1, stderr: "GO-2026-0001\n"}, false},
		{"refusal", "commit-msg", result{code: 1, stderr: "golaw commit-msg: commit.conventional\n"}, false},
		{"execution error", "pre-commit", result{err: ErrAcceptance}, false},
	}
	for _, tt := range tests {
		if measured := m2MeasuredReason(tt.rung, tt.got) == ""; measured != tt.measured {
			t.Fatalf("%s: measured=%v, want %v", tt.name, measured, tt.measured)
		}
	}
}

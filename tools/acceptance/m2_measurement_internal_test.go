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
	"strings"
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
// not judge, the line of files over the read bound and every line that
// leaves work to the pipeline, save the line of a step this version never
// runs on the machine, and names the whole line it refuses. It also
// requires the line each rung prints when its work ran to the end, so a
// silent rung, one that is not armed or one that skipped its Go steps,
// never times as a completion.
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
	statuses := regexp.MustCompile(`^\S+ unmeasured |govulncheck could not judge it|over the read bound, not judged here|left to the pipeline|the pipeline judges|not judged against a base`)
	pipeline := m2PipelineStep()
	for line := range strings.Lines(text) {
		if statuses.MatchString(line) && !pipeline.MatchString(line) {
			return "unmeasured: " + strings.TrimSpace(line)
		}
	}
	if !m2Completed(rung).MatchString(text) {
		return "no line shows the " + rung + " work ran to the end"
	}
	advisory := regexp.MustCompile(`GO-\d{4}-\d{4,}`)
	if got.code == 0 || rung == "pre-push" && got.code > 0 && advisory.MatchString(text) {
		return ""
	}
	return "exit " + strconv.Itoa(got.code)
}

// m2PipelineStep matches the line of a step of section 7 this version never
// runs on the machine and leaves to the make target of its name (D13). The
// rung prints it on every commit that stages the sources of the step, such
// as generate-check on any staged Go source once init has written
// .mockery.yaml, so the line costs the rung nothing and leaves none of the
// work it does run undone, unlike Go steps left to the pipeline.
func m2PipelineStep() *regexp.Regexp {
	return regexp.MustCompile(`^golaw pre-commit: (?:migrate-lint: left to the pipeline, make migrate-lint|` +
		`openapi-breaking: left to the pipeline, make openapi-breaking|` +
		`generate-check: left to the pipeline, make generate-check) judges the staged `)
}

// m2Completed matches the line a rung prints when its work ran to the end:
// go vet over the affected packages of the staged Go change, the verdict of
// the message, and the verdict of govulncheck on a pushed tip.
func m2Completed(rung string) *regexp.Regexp {
	switch rung {
	case "pre-commit":
		return regexp.MustCompile(`vet: go vet found nothing in \d+ affected packages`)
	case "commit-msg":
		return regexp.MustCompile(`verdict \w+: \d+ findings`)
	default:
		return regexp.MustCompile(`govulncheck finds no reachable vulnerability|GO-\d{4}-\d{4,} (reachable|waived)`)
	}
}

func TestM2MeasuredStatuses(t *testing.T) {
	t.Parallel()
	clean := "verdict clean: 0 findings (0 enforced, 0 notes), 32 excluded, 0 unmeasured\n"
	vetted := clean + "golaw pre-commit: vet: go vet found nothing in 8 affected packages\n"
	generator := "golaw pre-commit: generate-check: left to the pipeline, make generate-check judges the staged generator sources\n"
	tests := []struct {
		name     string
		rung     string
		got      result
		measured bool
	}{
		{"zero unmeasured", "pre-commit", result{stdout: vetted}, true},
		{"positive unmeasured", "pre-commit", result{
			stdout: "verdict clean: 0 findings (0 enforced, 0 notes), 31 excluded, 1 unmeasured\n",
		}, false},
		{"named unmeasured check", "pre-commit", result{
			stdout: "goast.file-length unmeasured the file is over the read bound\n" + vetted,
		}, false},
		{"scanner failure", "pre-push", result{
			stderr: "golaw pre-push: main: govulncheck could not judge it, the pipeline judges it: exit status 2\n",
		}, false},
		{"over the read bound", "pre-commit", result{
			stdout: vetted + "golaw pre-commit: over the read bound, not judged here: big.bin; the pipeline judges them\n",
		}, false},
		{"ignored Go files", "pre-commit", result{
			stdout: clean + "golaw pre-commit: tree: the Go steps are left to the pipeline: the worktree holds Go files git ignores, which the commit does not record and a copy of the index lacks: gen.go\n",
		}, false},
		{"a copy over its bounds", "pre-commit", result{
			stdout: clean + "golaw pre-commit: tree: the Go steps are left to the pipeline: tree over its bounds; stage the whole change, so the worktree holds what the index holds, and they read it in place\n",
		}, false},
		{"a base it could not measure", "pre-commit", result{
			stdout: vetted + "golaw pre-commit: not judged against a base it could not measure, the pipeline judges them: goast.sentinel-decl (parse a/a.go)\n",
		}, false},
		{"a step this version never runs", "pre-commit", result{
			stdout: vetted + generator,
		}, true},
		{"every step this version never runs", "pre-commit", result{
			stdout: vetted + generator +
				"golaw pre-commit: migrate-lint: left to the pipeline, make migrate-lint judges the staged migrations\n" +
				"golaw pre-commit: openapi-breaking: left to the pipeline, make openapi-breaking judges the staged contract\n",
		}, true},
		{"formatters over whole files", "pre-commit", result{
			stdout: vetted + "golaw pre-commit: fmt: golangci-lint fmt would change the staged Go files as whole files; the lint step judges the formatters on the staged lines, the pipeline the rest\n",
		}, true},
		{"linters left to the pipeline", "pre-commit", result{
			stdout: vetted + "golaw pre-commit: lint: left to the pipeline, golangci-lint cannot load scripts/lint/rules.go while go.mod does not require github.com/quasilyte/go-ruleguard/dsl\n",
		}, false},
		{"a step never run beside Go steps left to the pipeline", "pre-commit", result{
			stdout: clean + generator + "golaw pre-commit: tree: the Go steps are left to the pipeline: tree over its bounds\n",
		}, false},
		{"a step never run named by another rung", "pre-commit", result{
			stdout: vetted + "golaw pre-push: generate-check: left to the pipeline, make generate-check judges the staged generator sources\n",
		}, false},
		{"silent pre-commit", "pre-commit", result{}, false},
		{"silent commit-msg", "commit-msg", result{}, false},
		{"silent pre-push", "pre-push", result{}, false},
		{"commit-msg verdict", "commit-msg", result{stdout: clean}, true},
		{"clean push", "pre-push", result{stdout: "golaw pre-push: 4569d0892268: govulncheck finds no reachable vulnerability\n"}, true},
		{"measured advisory", "pre-push", result{code: 1, stderr: "golaw pre-push: main: GO-2026-0001 reachable in stdlib, fixed in v1.27.2\n"}, true},
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

// A refused sample names the whole line it read, so the step that left
// its work to the pipeline is the one the failure names.
func TestM2MeasuredReasonNamesTheLine(t *testing.T) {
	t.Parallel()
	line := "golaw pre-commit: tree: the Go steps are left to the pipeline: tree over its bounds"
	got := m2MeasuredReason("pre-commit", result{
		stdout: "golaw pre-commit: generate-check: left to the pipeline, make generate-check judges the staged generator sources\r\n" + line + "\r\n",
	})
	if got != "unmeasured: "+line {
		t.Fatalf("reason %q does not name the refused line", got)
	}
}

package main

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	scanreport "github.com/cristianargotti/golaw/internal/report"
)

func TestM2StagedRejectsIncompleteDecisions(t *testing.T) {
	t.Parallel()
	for _, got := range []result{
		{err: ErrAcceptance},
		{code: 2, stderr: "a different refusal"},
		{code: 3, stdout: "{}"},
		{stdout: "{"},
		{stdout: "{} {}"},
		{stdout: "null"},
		{stderr: "unexpected warning"},
	} {
		if err := m2JudgeStaged("partial-near", got); err == nil {
			t.Fatalf("incomplete decision passed: %+v", got)
		}
	}
	for _, tt := range []struct {
		name   string
		mutate func(*scanreport.Document)
	}{
		{"schema", func(d *scanreport.Document) { d.Schema = 2 }},
		{"mode", func(d *scanreport.Document) { d.Mode = "worktree" }},
		{"version", func(d *scanreport.Document) { d.KitVersion = "0.1.0" }},
		{"count", func(d *scanreport.Document) { d.Counts.Unmeasured = 1 }},
		{"status", func(d *scanreport.Document) {
			d.Checks = []scanreport.Status{{ID: "text.dashes", Status: "unmeasured"}}
		}},
		{"verdict", func(d *scanreport.Document) { d.Verdict = "fail" }},
		{"findings", func(d *scanreport.Document) { d.Findings = []model.Finding{m2StagedFinding("partial")} }},
		{"finding count", func(d *scanreport.Document) { d.Counts.Findings = 1 }},
		{"enforced count", func(d *scanreport.Document) { d.Counts.Enforced = 1 }},
	} {
		got := m2ChangedDecision(t, "partial-near", false, tt.mutate)
		if err := m2JudgeStaged("partial-near", got); err == nil {
			t.Fatalf("%s: incomplete clean report passed", tt.name)
		}
	}
}

func TestM2StagedRequiresTheIntroducedFinding(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name   string
		mutate func(*scanreport.Document)
	}{
		{"verdict", func(d *scanreport.Document) { d.Verdict = "clean" }},
		{"enforced count", func(d *scanreport.Document) { d.Counts.Enforced = 0 }},
		{"absent", func(d *scanreport.Document) { d.Findings = nil }},
		{"check", func(d *scanreport.Document) { d.Findings[0].Check = "another.check" }},
		{"path", func(d *scanreport.Document) { d.Findings[0].Path = "another.txt" }},
		{"note", func(d *scanreport.Document) { d.Findings[0].Severity = model.SeverityNote }},
	} {
		got := m2ChangedDecision(t, "partial-red", true, tt.mutate)
		if err := m2JudgeStaged("partial-red", got); err == nil {
			t.Fatalf("%s: missing enforced finding passed", tt.name)
		}
	}
	got := m2StagedDocument(t, "partial-red", true)
	got.code = 0
	if err := m2JudgeStaged("partial-red", got); err == nil {
		t.Fatal("red report with exit zero passed")
	}
}

func TestM2PolicyRefusalRequiresItsRepair(t *testing.T) {
	t.Parallel()
	for _, got := range []result{
		{code: 1, stderr: m2PolicyRefusal},
		{code: 2, stdout: "{}", stderr: m2PolicyRefusal},
		{code: 2},
		{code: 2, stderr: strings.ReplaceAll(m2PolicyRefusal, "file_lines", "other")},
		{code: 2, stderr: strings.ReplaceAll(m2PolicyRefusal, "restore", "other")},
		{code: 2, stderr: strings.ReplaceAll(m2PolicyRefusal, "unstage golaw.yml", "other")},
	} {
		if err := m2JudgeStaged("policy-red", got); err == nil {
			t.Fatalf("unactionable policy refusal passed: %+v", got)
		}
	}
}

func TestM2StagedPropagatesPreparationAndObservationFailures(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"unknown", "preparation", "before", "after"} {
		s := m2TestSuite(t)
		script := m2StagedScript(t, s, "partial-near", "answer")
		s.execute = func(ctx context.Context, c command) result {
			if mode == "preparation" ||
				(c.program == s.git && strings.Contains(strings.Join(c.args, " "), "rev-parse") &&
					(mode == "before" || mode == "after" && script.Targets > 0)) {
				return result{err: ErrAcceptance}
			}
			return script.Execute(ctx, c)
		}
		name := "partial-near"
		if mode == "unknown" {
			name = "unknown"
		}
		if err := s.stagedM2(t.Context(), name); !errors.Is(err, ErrAcceptance) {
			t.Fatalf("%s: failure = %v", mode, err)
		}
	}
}

func m2ChangedDecision(t *testing.T, name string, red bool, mutate func(*scanreport.Document)) result {
	t.Helper()
	got := m2StagedDocument(t, name, red)
	var doc scanreport.Document
	if err := json.Unmarshal([]byte(got.stdout), &doc); err != nil {
		t.Fatal(err)
	}
	mutate(&doc)
	body, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	got.stdout = string(body)
	return got
}

func TestM2StagedDetectsAdministrativeMutation(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"head", "selected-index", "default-index"} {
		s := m2TestSuite(t)
		script := m2StagedScript(t, s, "alternate-near", "answer")
		s.execute = func(ctx context.Context, c command) result {
			got := script.Execute(ctx, c)
			if c.program == s.binary && slices.Contains(c.args, "--staged") {
				m2AdministrativeMutation(ctx, t, s, c, mode)
			}
			return got
		}
		if err := s.stagedM2(t.Context(), "alternate-near"); err == nil ||
			!strings.Contains(err.Error(), "changed repository state") {
			t.Fatalf("%s: mutation error = %v", mode, err)
		}
	}
}

func m2AdministrativeMutation(ctx context.Context, t *testing.T, s *suite, c command, mode string) {
	t.Helper()
	fixture := m2FixtureExecutor(t, s)
	c.program = s.git
	if mode == "head" {
		c.args = append(commitArgs("test: simulate a scan mutation"), "--allow-empty")
		if err := success(fixture(ctx, c)); err != nil {
			t.Fatal(err)
		}
		return
	}
	if mode == "default-index" {
		c.env = slices.DeleteFunc(slices.Clone(c.env), func(entry string) bool {
			key, _, _ := strings.Cut(entry, "=")
			return strings.EqualFold(key, "GIT_INDEX_FILE")
		})
	}
	c.args = []string{"rev-parse", "--git-path", "index"}
	path := fixture(ctx, c)
	if err := success(path); err != nil {
		t.Fatal(err)
	}
	index := strings.TrimSpace(path.stdout)
	if !filepath.IsAbs(index) {
		index = filepath.Join(c.dir, index)
	}
	body, err := readFile(index)
	if err != nil {
		t.Fatal(err)
	}
	version := "4"
	if len(body) > 7 && body[7] == 4 {
		version = "2"
	}
	c.args = []string{"update-index", "--index-version=" + version}
	if err := success(fixture(ctx, c)); err != nil {
		t.Fatal(err)
	}
}

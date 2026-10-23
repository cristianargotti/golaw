package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/policy"
)

func TestM5DashNotesHoldOneTypographicDash(t *testing.T) {
	t.Parallel()
	if got := strings.Count(string(m5DashNotes()), string(rune(0x2014))); got != 1 {
		t.Fatalf("the note holds %d dashes, want one", got)
	}
}

func TestM5JudgeSeededHoldsTheEntryAndItsHorizon(t *testing.T) {
	t.Parallel()
	entry := policy.ExceptionRule{Check: "text.dashes", Path: m5DebtNotes, Record: m5DebtRecord, Until: "2027-04-19"}
	rows := []struct {
		name string
		file policy.File
		pass bool
	}{
		{"near 180 days", policy.File{Exceptions: []policy.ExceptionRule{entry}}, true},
		{"red no entry", policy.File{}, false},
		{"red two entries", policy.File{Exceptions: []policy.ExceptionRule{entry, entry}}, false},
		{"red another path", policy.File{Exceptions: []policy.ExceptionRule{{Check: entry.Check, Path: "other.md", Record: entry.Record, Until: entry.Until}}}, false},
		{"red no record", policy.File{Exceptions: []policy.ExceptionRule{{Check: entry.Check, Path: entry.Path, Until: entry.Until}}}, false},
		{"red a year", policy.File{Exceptions: []policy.ExceptionRule{{Check: entry.Check, Path: entry.Path, Record: entry.Record, Until: "2027-10-21"}}}, false},
		{"red no date", policy.File{Exceptions: []policy.ExceptionRule{{Check: entry.Check, Path: entry.Path, Record: entry.Record, Until: "soon"}}}, false},
	}
	for _, row := range rows {
		if _, err := m5JudgeSeeded(row.file, "2026-10-21"); (err == nil) != row.pass {
			t.Errorf("%s: %v", row.name, err)
		}
	}
	data := []byte("exceptions:\n  - { check: text.dashes, path: notes.md, record: r, until: 2027-04-19 }\n")
	later, err := m5ShiftUntil(data, "2027-04-19", 30)
	if err != nil || !strings.Contains(string(later), "until: 2027-05-19") {
		t.Fatalf("later = %q, %v", later, err)
	}
	if _, err := m5ShiftUntil(data, "2030-01-01", 1); !errors.Is(err, ErrAcceptance) {
		t.Fatalf("an absent day = %v", err)
	}
	if _, err := m5ShiftUntil(append(data, data...), "2027-04-19", 1); !errors.Is(err, ErrAcceptance) {
		t.Fatalf("a day named twice = %v", err)
	}
	for _, row := range []struct {
		got  result
		pass bool
	}{
		{result{code: 2, stderr: "golaw: scan: proposed golaw.yml " + m5Weakens + "[text.dashes at notes.md].until: a to b\n"}, true},
		{result{code: 0}, false},
		{result{code: 2, stdout: "{}", stderr: m5Weakens}, false},
		{result{code: 2, stderr: "golaw: scan: other\n"}, false},
	} {
		if err := m5JudgeBroadening(row.got); (err == nil) != row.pass {
			t.Errorf("broadening %+v: %v", row.got, err)
		}
	}
}

// m5DebtReplies are the answers of the debt rows: init seeds once from
// the day of the last commit, the staged scan and the armed rung judge
// the index against HEAD through the tightening rule itself.
func m5DebtReplies(t *testing.T, s *suite) []m5Reply {
	t.Helper()
	show := func(t *testing.T, dir, revision string) policy.File {
		t.Helper()
		got := runCommand(t.Context(), command{program: s.git, args: []string{"show", revision + ":" + policyFile}, dir: dir, env: s.env})
		file, err := policy.Decode([]byte(got.stdout))
		if got.code != 0 || err != nil {
			t.Fatalf("show %s: %+v %v", revision, got, err)
		}
		return file
	}
	judge := func(code int) func(*testing.T, command) result {
		return func(t *testing.T, c command) result {
			t.Helper()
			if err := policy.CheckTightening(show(t, c.dir, "HEAD"), show(t, c.dir, ""), nil); err != nil {
				return result{code: code, stderr: "golaw: " + err.Error() + "\n"}
			}
			return result{}
		}
	}
	return []m5Reply{
		{prefix: []string{commandInit, m5SeedFlag}, answer: func(t *testing.T, c command) result {
			t.Helper()
			path := filepath.Join(c.dir, policyFile)
			if _, err := os.Stat(path); err == nil {
				return result{code: 2, stderr: "golaw: init: the debt set is seeded once (A24); remove entries from golaw.yml, never add them\n"}
			}
			m5FakeInit(t, c)
			day := strings.TrimSpace(runCommand(t.Context(), command{program: s.git, args: []string{"log", "-1", "--format=%cs"}, dir: c.dir, env: s.env}).stdout)
			start, err := time.Parse(time.DateOnly, day)
			if err != nil {
				t.Fatal(err)
			}
			until := start.AddDate(0, 0, 180).Format(time.DateOnly)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			seeded := strings.Replace(string(data), "exceptions: []", "exceptions:\n  - { check: text.dashes, path: notes.md, record: "+m5DebtRecord+", until: "+until+" }", 1)
			m2Write(t, path, seeded)
			m2Write(t, filepath.Join(c.dir, filepath.FromSlash(m5DebtRecord)), "# Inherited debt\n\n- text.dashes at notes.md until "+until+"\n")
			m5FakeEnroll(t, c)
			return result{}
		}},
		{prefix: []string{commandInit}, answer: m5Answer(result{})},
		{prefix: []string{commandScan, "--staged"}, answer: judge(2)},
		{prefix: []string{commandScan}, answer: m5Answer(result{})},
		{prefix: []string{"setup"}, answer: m5Answer(result{})},
		{prefix: []string{commandHook, "pre-commit"}, answer: judge(1)},
	}
}

func TestM5DebtRowsWithASimulatedCandidate(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"debt-seeded-once", "debt-broadening-refused"} {
		c := m5AdoptionCases()[slices.Index(m5Names(m5AdoptionCases()), name)]
		s := m2TestSuite(t)
		scope := s.m5Scope(m5AdoptionCategory, name)
		scope.execute = m5Script(t, scope, m5DebtReplies(t, scope)...).Execute
		if err := c.run(scope, t.Context()); err != nil {
			t.Errorf("%s: %v", name, err)
		}
		lenient := s.m5Scope(m5AdoptionCategory, name+"-lenient")
		replies := append([]m5Reply{
			{prefix: []string{commandInit, m5SeedFlag, flagApply}, once: true, answer: m5DebtReplies(t, lenient)[0].answer},
			{prefix: []string{commandInit, m5SeedFlag}, answer: m5Answer(result{})},
			{prefix: []string{commandScan, "--staged"}, answer: m5Answer(result{})},
		}, m5DebtReplies(t, lenient)...)
		lenient.execute = m5Script(t, lenient, replies...).Execute
		if err := c.run(lenient, t.Context()); err == nil {
			t.Errorf("%s passed over a candidate that seeds twice and accepts a broadening", name)
		}
	}
}

func TestM5DebtRowsReportEveryFailedStep(t *testing.T) {
	t.Parallel()
	for _, c := range m5AdoptionCases()[4:] {
		m5Sweep(t, m5AdoptionCategory, c, m5DebtReplies)
	}
}

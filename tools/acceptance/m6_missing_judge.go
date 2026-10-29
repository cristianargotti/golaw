package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/doctor"
	scanreport "github.com/cristianargotti/golaw/internal/report"
)

// The wraps every judge of the missing tool rows shares.
const (
	m6RunCandidate = "run the candidate: %w"
	m6DecodeDoctor = "decode doctor --json: %w"
)

// m6DoctorLinesOf reads every line of doctor --json, which prints one
// document per agent, so a surface line is found whichever document holds it.
func m6DoctorLinesOf(stdout string) ([]doctor.Line, error) {
	decoder := json.NewDecoder(strings.NewReader(stdout))
	var lines []doctor.Line
	for {
		var report doctor.Report
		err := decoder.Decode(&report)
		if errors.Is(err, io.EOF) {
			return lines, nil
		}
		if err != nil {
			return nil, fmt.Errorf(m6DecodeDoctor, err)
		}
		lines = append(lines, report.Lines...)
	}
}

// m6JudgeDoctor requires one tool line per tool a language of the
// repository needs, missing or available as the kit's own resolution says,
// and no line for a tool no language needs, so a doctor that names every
// pinned tool on every repository is refused as noise.
func m6JudgeDoctor(lines []doctor.Line, want map[string]bool) error {
	counts, codes := make(map[string]int), make(map[string]string)
	for _, line := range lines {
		if line.Code != m6ToolMissing && line.Code != m6ToolAvailable {
			continue
		}
		if _, needed := want[line.Key]; !needed {
			return fmt.Errorf("doctor names %s, a tool no language of the repository needs: %w", line.Key, ErrAcceptance)
		}
		counts[line.Key]++
		codes[line.Key] = line.Code
	}
	for _, name := range slices.Sorted(maps.Keys(want)) {
		if counts[name] != 1 {
			return fmt.Errorf("doctor prints %d lines for %s, want one: %w", counts[name], name, ErrAcceptance)
		}
		expected := m6ToolMissing
		if want[name] {
			expected = m6ToolAvailable
		}
		if codes[name] != expected {
			return fmt.Errorf("doctor calls %s %s, want %s: %w", name, codes[name], expected, ErrAcceptance)
		}
	}
	return nil
}

// m6JudgeNotice requires a post event answer that allows the write and
// hands the agent the missing tool sentence.
func m6JudgeNotice(got result, notice string) error {
	if got.err != nil {
		return fmt.Errorf(m6RunCandidate, got.err)
	}
	if got.code != 0 || got.stderr != "" {
		return fmt.Errorf("the post event answered exit %d %q, want 0 with no stderr: %w", got.code, strings.TrimSpace(got.stderr), ErrAcceptance)
	}
	var decoded answer
	if err := json.Unmarshal([]byte(got.stdout), &decoded); err != nil {
		return fmt.Errorf("decode the post event answer: %w", err)
	}
	if decoded.Decision == decisionBlock {
		return fmt.Errorf("a missing tool blocked the write: %s: %w", decoded.Reason, ErrAcceptance)
	}
	if decoded.Hook.Event != eventPost {
		return fmt.Errorf("the answer names the event %q, want %s: %w", decoded.Hook.Event, eventPost, ErrAcceptance)
	}
	if !strings.Contains(decoded.Hook.Additional, notice) {
		return fmt.Errorf("the agent context %q lacks the notice %q: %w", decoded.Hook.Additional, notice, ErrAcceptance)
	}
	return nil
}

// m6JudgeRung requires the armed pre-commit rung to let the commit through
// with either sentence of the notice, since M6.5.3 names no channel for it.
func m6JudgeRung(got result, agent, person string) error {
	if got.err != nil {
		return fmt.Errorf(m6RunCandidate, got.err)
	}
	if got.code != 0 {
		return fmt.Errorf("the armed rung refused the commit: exit %d %q: %w", got.code, strings.TrimSpace(got.stderr), ErrAcceptance)
	}
	printed := got.stdout + got.stderr
	if !strings.Contains(printed, agent) && !strings.Contains(printed, person) {
		return fmt.Errorf("the armed rung printed no notice: %w", ErrAcceptance)
	}
	return nil
}

// m6JudgeUnmeasured requires lint to fail as unmeasured, exit 2, with every
// missing tool's check unmeasured and its reason naming the tool, so the
// person knows which install the pipeline still lacks.
func m6JudgeUnmeasured(got result, missing []string) error {
	if got.err != nil {
		return fmt.Errorf(m6RunCandidate, got.err)
	}
	if got.code != 2 {
		return fmt.Errorf("lint exit %d, want 2 for a missing tool: %w", got.code, ErrAcceptance)
	}
	doc, err := m6Document(got.stdout)
	if err != nil {
		return err
	}
	for _, name := range missing {
		index := slices.IndexFunc(doc.Checks, func(c scanreport.Status) bool { return c.ID == m6LintPrefix+name })
		if index < 0 {
			return fmt.Errorf("lint reports no %s%s check: %w", m6LintPrefix, name, ErrAcceptance)
		}
		check := doc.Checks[index]
		if check.Status != m6Unmeasured || !strings.Contains(check.Reason, name) {
			return fmt.Errorf("%s is %s %q, want unmeasured naming %s: %w", check.ID, check.Status, check.Reason, name, ErrAcceptance)
		}
	}
	return nil
}

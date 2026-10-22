package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/cristianargotti/golaw/internal/doctor"
)

const (
	categoryInstallation = "installation"
	caseDoctor           = "doctor-json"
	repoInstallation     = "installation"
	lineArtifacts        = "doctor.artifacts"
	lineBinary           = "doctor.binary"
	lineHooks            = "doctor.hooks"
	lineBuild            = "doctor.build"
	lineDispatch         = "doctor.dispatch"
	receiptFile          = "receipt.json"
)

// doctorRow replays item 2 of section 10.3: once an installed hook ran in
// the installed repository, doctor --json decodes, every artifact the
// receipt lists matches, every hook of its definition blocks on failure,
// the hook names the binary this run built, a source build is metadata,
// never verified (section 11 M1), and the hook process left the dispatch
// evidence of section 5.6.
func (s *suite) doctorRow(ctx context.Context) row {
	repo := filepath.Join(s.base, dirRepos, repoInstallation)
	input, err := payload(s.root, repo, writeFixture(caseDoctor, pathNote, textSafe, wantAllow))
	if err == nil {
		err = expect(s.invoke(ctx, repo, eventPre, toolWrite, input), wantAllow)
	}
	if err != nil {
		return measured(categoryInstallation, caseDoctor, err)
	}
	got := s.execute(ctx, command{
		program: s.binary, args: []string{"doctor", "--json"}, dir: repo, env: s.env,
	})
	if err := success(got); err != nil {
		return measured(categoryInstallation, caseDoctor, err)
	}
	var report doctor.Report
	decoder := json.NewDecoder(bytes.NewReader([]byte(got.stdout)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&report); err != nil {
		return measured(categoryInstallation, caseDoctor, fmt.Errorf("decode doctor --json: %w", err))
	}
	owed, readErr := receiptKeys(s.home)
	if readErr != nil {
		return measured(categoryInstallation, caseDoctor, readErr)
	}
	return measured(categoryInstallation, caseDoctor, judgeDoctor(report, owed, s.binary))
}

// owedLines come from the receipt the installer wrote, one key per artifact
// and per event and matcher of its hook definition, rather than from
// doctor, so a line doctor leaves out is missing evidence and never a
// smaller report.
type owedLines struct {
	artifacts []string
	hooks     []string
}

// receiptKeys reads the one receipt under the isolated home, so the row
// holds doctor to what the installer recorded rather than to what doctor
// chose to print.
func receiptKeys(home string) (owedLines, error) {
	path, err := findReceipt(home)
	if err != nil {
		return owedLines{}, err
	}
	body, err := readFile(path)
	if err != nil {
		return owedLines{}, err
	}
	var receipt struct {
		Artifacts []struct {
			Path string `json:"path"`
		} `json:"artifacts"`
		HookDefinition struct {
			Hooks map[string][]hookGroup `json:"hooks"`
		} `json:"hook_definition"`
	}
	if err := json.Unmarshal(body, &receipt); err != nil {
		return owedLines{}, fmt.Errorf("decode the receipt: %w", err)
	}
	var owed owedLines
	for _, artifact := range receipt.Artifacts {
		owed.artifacts = append(owed.artifacts, artifact.Path)
	}
	for event, groups := range receipt.HookDefinition.Hooks {
		for _, group := range groups {
			owed.hooks = append(owed.hooks, event+"/"+group.Matcher)
		}
	}
	if len(owed.artifacts) == 0 || len(owed.hooks) == 0 {
		return owedLines{}, fmt.Errorf("the receipt lists no artifact or no hook: %w", ErrAcceptance)
	}
	return owed, nil
}

// findReceipt answers the only Claude receipt under home: the agents
// installed beside it keep receipts of their own, which name their agent,
// and a second Claude receipt means the row cannot tell which installation
// doctor reported.
func findReceipt(home string) (string, error) {
	var found []string
	err := filepath.WalkDir(home, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || entry.Name() != receiptFile {
			return walkErr
		}
		agent, err := receiptAgent(path)
		if err == nil && agent == agentClaude {
			found = append(found, path)
		}
		return err
	})
	if err != nil {
		return "", fmt.Errorf("find the receipt: %w", err)
	}
	if len(found) != 1 {
		return "", fmt.Errorf("found %d receipts of %s under the isolated home: %w", len(found), agentClaude, ErrAcceptance)
	}
	return found[0], nil
}

// receiptAgent reads the agent a receipt names; a receipt without one is
// the Claude receipt every installation before M4 wrote.
func receiptAgent(path string) (string, error) {
	body, err := readFile(path)
	if err != nil {
		return "", err
	}
	var receipt struct {
		Agent string `json:"agent"`
	}
	if err := json.Unmarshal(body, &receipt); err != nil {
		return "", fmt.Errorf("decode the receipt %s: %w", path, err)
	}
	if receipt.Agent == "" {
		return agentClaude, nil
	}
	return receipt.Agent, nil
}

// judgedLine is the code and the state a judged line must carry together:
// a code under another state is a contradiction, not evidence.
type judgedLine struct{ code, state string }

func judgedLines() map[string]judgedLine {
	return map[string]judgedLine{
		lineArtifacts: {"artifact-matches", "ok"},
		lineBinary:    {"binary-matches", "ok"},
		lineHooks:     {"hook-failure-blocked", "ok"},
		lineBuild:     {"source-build-unverifiable", "source-build"},
		lineDispatch:  {"dispatch-observed", "ok"},
	}
}

// judgeDoctor holds doctor --json to item 2 of section 10.3: schema 1,
// every judged line at its code and state, one line for every artifact and
// hook of the receipt and none beyond them, one build line, and one binary
// line naming the binary this run built.
func judgeDoctor(report doctor.Report, owed owedLines, built string) error {
	if report.Schema != 1 {
		return fmt.Errorf("doctor --json schema is %d: %w", report.Schema, ErrAcceptance)
	}
	lines, err := judgeLines(report.Lines)
	if err != nil {
		return err
	}
	for id := range judgedLines() {
		if len(lines[id]) == 0 {
			return fmt.Errorf("doctor --json lacks a line of section 10.3 (%s): %w", id, ErrAcceptance)
		}
	}
	if len(lines[lineBinary]) != 1 || len(lines[lineBuild]) != 1 {
		return fmt.Errorf("doctor --json repeats the binary or the build line: %w", ErrAcceptance)
	}
	if err := sameKeys(lineArtifacts, lines[lineArtifacts], owed.artifacts); err != nil {
		return err
	}
	if err := sameKeys(lineHooks, lines[lineHooks], owed.hooks); err != nil {
		return err
	}
	return judgeBinary(lines[lineBinary][0], built)
}

// judgeLines refuses a judged line before it counts it, so a line at the
// wrong code or state is never taken as coverage.
func judgeLines(all []doctor.Line) (map[string][]doctor.Line, error) {
	wants := judgedLines()
	out := make(map[string][]doctor.Line, len(wants))
	for _, line := range all {
		want, judged := wants[line.ID]
		if !judged {
			continue
		}
		if line.Code != want.code || line.State != want.state {
			return nil, fmt.Errorf("doctor %s %s is %s %s (%s): %w",
				line.ID, line.Key, line.State, line.Code, line.Detail, ErrAcceptance)
		}
		out[line.ID] = append(out[line.ID], line)
	}
	return out, nil
}

// sameKeys compares sorted keys, so a repeated line cannot stand in for a
// missing one.
func sameKeys(id string, lines []doctor.Line, owed []string) error {
	got := make([]string, 0, len(lines))
	for _, line := range lines {
		got = append(got, line.Key)
	}
	slices.Sort(got)
	want := slices.Sorted(slices.Values(owed))
	if !slices.Equal(got, want) {
		return fmt.Errorf("doctor %s lines %v, the receipt lists %v: %w", id, got, want, ErrAcceptance)
	}
	return nil
}

// judgeBinary requires the binary line to name the binary this run built,
// compared as files because the agent resolves it through its own search
// path, where a linked temporary directory spells it another way.
func judgeBinary(line doctor.Line, built string) error {
	named, err := os.Stat(line.Detail)
	if err != nil {
		return fmt.Errorf("doctor binary line names %q: %w", line.Detail, err)
	}
	ours, err := os.Stat(built)
	if err != nil {
		return fmt.Errorf("inspect the built binary: %w", err)
	}
	if !os.SameFile(named, ours) {
		return fmt.Errorf("doctor binary line names %s, not the built %s: %w", line.Detail, built, ErrAcceptance)
	}
	return nil
}

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/cristianargotti/golaw/internal/hook/codex"
)

// The live lane of D21: the flag that runs it, the variable codex exec
// reads its key from (CODEX_API_KEY, honored by codex exec at 0.163.0),
// the repository it prepares, the two targets its sessions write and the
// bound of one session.
const (
	m4LaneFlag      = "live-codex"
	m4LaneVariable  = "CODEX_API_KEY"
	m4LaneRow       = "codex-lane"
	m4LaneRepo      = "codex-live"
	m4DeniedTarget  = "notes/denied.md"
	m4ControlTarget = "notes/allowed.md"
	m4ControlLine   = "one-two"
	m4LaneTimeout   = 10 * time.Minute
	m4HookBlock     = "blocked by PreToolUse hook"
)

// m4LaneFacts are the facts of the person who runs the lane, the ones
// every live record keeps beside what the lane observes; the date enters
// with them, because the harness reads no clock.
type m4LaneFacts struct {
	Date               string `json:"date"`
	Owner              string `json:"owner"`
	Environment        string `json:"environment"`
	CredentialBoundary string `json:"credential_boundary"`
	SpendingLimit      string `json:"spending_limit"`
	Schedule           string `json:"schedule"`
	Retention          string `json:"retention"`
}

// runCodexLane records the Codex live session and reports its one row:
// PASS with the record written, SKIP with the reason the lane cannot run
// unattended here, FAIL when a step or a case fails.
func (s *suite) runCodexLane(ctx context.Context, out io.Writer, factsPath string) int {
	item := s.codexLane(ctx, factsPath, os.Getenv(m4LaneVariable), runtime.GOOS+"/"+runtime.GOARCH)
	evidence := map[string]rowEvidence{categoryLive + "/" + m4LaneRow: {Agent: agentCodex, Version: codex.PatchGrammar, Binary: s.digest}}
	return m2ReportCode(reportEvidence(out, []row{item}, "", s.digest, evidence))
}

// codexLane runs the lane on a platform with a credential, the value never
// printed and never written.
func (s *suite) codexLane(ctx context.Context, factsPath, credential, platform string) row {
	switch {
	case platform != liveOS+"/"+liveArch:
		return row{categoryLive, m4LaneRow, statusSkip, "the Codex live record is a session on darwin/arm64, not " + platform, false}
	case credential == "":
		return row{categoryLive, m4LaneRow, statusSkip, m4LaneVariable + " is not set and the isolated home holds no credential, so the lane cannot run unattended: follow " + m4LiveProcedure, false}
	}
	facts, err := readLaneFacts(factsPath)
	if err != nil {
		return measured(categoryLive, m4LaneRow, err)
	}
	if toolsErr := s.tools(); toolsErr != nil {
		return measured(categoryLive, m4LaneRow, toolsErr)
	}
	codexPath, reason := s.m4PinnedCodex(ctx)
	if reason != "" {
		return row{categoryLive, m4LaneRow, statusSkip, reason, false}
	}
	record, err := s.m4LaneRecord(ctx, codexPath, credential, facts)
	item := measured(categoryLive, m4LaneRow, err)
	if err == nil {
		item.detail = "live record of " + record.Date + " written to " + m4LiveRecordPath + ": review it and commit it alone"
	}
	return item
}

// readLaneFacts reads the facts file strictly, every fact a sentence.
func readLaneFacts(path string) (m4LaneFacts, error) {
	body, err := readFile(path)
	if err != nil {
		return m4LaneFacts{}, err
	}
	var facts m4LaneFacts
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&facts); err != nil {
		return m4LaneFacts{}, fmt.Errorf(decodeFormat, path, err)
	}
	values := []string{facts.Date, facts.Owner, facts.Environment, facts.CredentialBoundary, facts.SpendingLimit, facts.Schedule, facts.Retention}
	if slices.ContainsFunc(values, func(value string) bool { return strings.TrimSpace(value) == "" }) {
		return m4LaneFacts{}, fmt.Errorf("the facts file %s misses a fact: %w", path, ErrAcceptance)
	}
	return facts, nil
}

// m4LaneRecord prepares the lane, runs both sessions and writes the record
// only when both cases hold.
func (s *suite) m4LaneRecord(ctx context.Context, codexPath, credential string, facts m4LaneFacts) (liveRecord, error) {
	t, digest, err := s.m4LanePrepare(ctx)
	if err != nil {
		return liveRecord{}, err
	}
	cases, err := s.m4LaneSessions(ctx, t, codexPath, credential)
	if err != nil {
		return liveRecord{}, err
	}
	record := liveRecord{
		Date: facts.Date, Agent: agentCodex, OS: liveOS, Arch: liveArch,
		AgentVersion: "codex-cli " + codex.PatchGrammar, Binary: digest,
		Owner: facts.Owner, Environment: facts.Environment, CredentialBoundary: facts.CredentialBoundary,
		SpendingLimit: facts.SpendingLimit, Schedule: facts.Schedule, Retention: facts.Retention, Cases: cases,
	}
	if validErr := validCodexRecord(record); validErr != nil {
		return liveRecord{}, validErr
	}
	body, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return liveRecord{}, fmt.Errorf("encode the live record: %w", err)
	}
	return record, writeFile(filepath.Join(s.root, filepath.FromSlash(m4LiveRecordPath)), append(body, '\n'), fileMode)
}

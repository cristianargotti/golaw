package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path/filepath"

	"github.com/cristianargotti/golaw/internal/hook/codex"
)

// The live rows of M4 (D21): the dated Codex session of macOS, its
// procedure, the two cases it records and the Cursor preview, which no
// lane records yet (cut 13).
const (
	m4LiveRecordPath  = "docs/live/codex-darwin.json"
	m4LiveProcedure   = "docs/live/codex.md"
	caseCodexDenied   = "codex-denied-write"
	caseCodexControl  = "codex-allowed-control"
	caseCursorPreview = "cursor-agent-print"
	targetAbsent      = "absent"
	targetPresent     = "present"
)

// m4LiveRows are the Codex live rows from the record of the module and the
// Cursor preview row. A missing or stale Codex record is an optional skip
// in every run, the release included: the record is what Codex support
// waits for (docs/agent-contracts.md), not a gate of the release, which
// docs/releasing.md holds to the Claude record. A record that is present
// and wrong still fails.
func (s *suite) m4LiveRows(ctx context.Context, _ bool) []row {
	rows := s.m4LiveRowsFrom(ctx, filepath.Join(s.root, filepath.FromSlash(m4LiveRecordPath)), false)
	return append(rows, row{
		categoryLive, caseCursorPreview, statusSkip,
		"no agent -p lane has recorded Cursor hooks firing: Cursor stays a compatibility preview (cut 13)", false,
	})
}

// m4LiveRowsFrom judges the Codex record at path as the M1 rows judge the
// Claude one: absent, or naming another binary, is SKIP, which a release
// refuses; a record that misses a fact or shows a guard failing is FAIL.
func (s *suite) m4LiveRowsFrom(ctx context.Context, path string, required bool) []row {
	record, err := decodeLiveRecord(path)
	if errors.Is(err, fs.ErrNotExist) {
		return m4LiveWith(statusSkip, "no dated live record at "+m4LiveRecordPath+
			": the lane needs a credential the isolated home of a run never holds, so a person runs it as "+
			m4LiveProcedure+" says", required)
	}
	if err == nil {
		err = validCodexRecord(record)
	}
	if err != nil {
		return m4LiveWith(statusFail, err.Error(), true)
	}
	digest, err := s.candidateDigest(ctx)
	if err != nil {
		return m4LiveWith(statusFail, err.Error(), true)
	}
	if record.Binary != digest {
		return m4LiveWith(statusSkip, fmt.Sprintf("the live record of %s names binary %s, the darwin/arm64 candidate is %s",
			record.Date, record.Binary, digest), required)
	}
	return m4LiveWith(statusPass, fmt.Sprintf("live record of %s by %s on %s", record.Date, record.Owner, record.AgentVersion), true)
}

func m4LiveWith(status, detail string, required bool) []row {
	return []row{
		{categoryLive, caseCodexDenied, status, detail, required},
		{categoryLive, caseCodexControl, status, detail, required},
	}
}

// m4CodexLiveCases are the two cases of the Codex session: the apply_patch
// denial leaves its file absent, the control's file is present.
func m4CodexLiveCases() map[string]liveCase {
	return map[string]liveCase{
		caseDenied:  {outcomeBlocked, targetAbsent},
		caseControl: {outcomeAllowed, targetPresent},
	}
}

// validCodexRecord holds a record to the Codex session of D21: Codex on
// darwin/arm64 at the version the adapter's grammar is pinned to, the
// facts of every record, and exactly the two cases.
func validCodexRecord(record liveRecord) error {
	if record.Agent != agentCodex || record.OS != liveOS || record.Arch != liveArch {
		return fmt.Errorf("the live record is not a Codex session on darwin/arm64: %w", ErrAcceptance)
	}
	if record.AgentVersion != "codex-cli "+codex.PatchGrammar {
		return fmt.Errorf("the live record ran %q, not the validated codex-cli %s: %w", record.AgentVersion, codex.PatchGrammar, ErrAcceptance)
	}
	if err := recordFacts(record); err != nil {
		return err
	}
	if !maps.Equal(record.Cases, m4CodexLiveCases()) {
		return fmt.Errorf("the live record holds %v, not the denied write with the target absent and the control present: %w", record.Cases, ErrAcceptance)
	}
	return nil
}

package main

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook/codex"
)

// validCodexLiveRecord is a complete Codex record naming the simulated
// candidate.
func validCodexLiveRecord() liveRecord {
	record := validRecord()
	record.Agent, record.AgentVersion = agentCodex, "codex-cli "+codex.PatchGrammar
	record.CredentialBoundary = "an API key with a spending limit, read from the shell of the lane only"
	record.Cases = m4CodexLiveCases()
	return record
}

func TestM4LiveRowsPassOnlyForTheCodexCandidate(t *testing.T) {
	t.Parallel()
	s := testSuite(t)
	for _, required := range []bool{false, true} {
		rows := s.m4LiveRows(t.Context(), required)
		if got := strings.Join(m4RowNames(rows), ","); got != "codex-denied-write SKIP,codex-allowed-control SKIP,cursor-agent-print SKIP" ||
			rows[0].required != required || rows[2].required || !strings.Contains(rows[1].detail, m4LiveProcedure) ||
			!strings.Contains(rows[2].detail, "compatibility preview") {
			t.Fatalf("required %v: %+v", required, rows)
		}
	}
	rows := s.m4LiveRowsFrom(t.Context(), writeRecord(t, validCodexLiveRecord()), true)
	if rows[0].status != statusPass || rows[1].name != caseCodexControl || !strings.Contains(rows[0].detail, "codex-cli "+codex.PatchGrammar) {
		t.Fatalf("a valid record = %+v", rows)
	}
	stale := validCodexLiveRecord()
	stale.Binary = strings.Repeat("0", len(stale.Binary))
	if rows := s.m4LiveRowsFrom(t.Context(), writeRecord(t, stale), false); rows[0].status != statusSkip || rows[0].required {
		t.Fatalf("a record of another binary = %+v", rows)
	}
}

func TestM4LiveRecordsThatShowAFailureFail(t *testing.T) {
	t.Parallel()
	s := testSuite(t)
	for reason, mutate := range map[string]func(*liveRecord){
		"not a Codex session":    func(r *liveRecord) { r.Agent = agentClaude },
		"not the validated":      func(r *liveRecord) { r.AgentVersion = "codex-cli " + codex.PatchGrammar + "-alpha.10.2" },
		"misses a fact":          func(r *liveRecord) { r.Owner = "" },
		"the live record date":   func(r *liveRecord) { r.Date = "21/10/2026" },
		"not the denied write":   func(r *liveRecord) { r.Cases[caseDenied] = liveCase{outcomeBlocked, targetPresent} },
		"control present":        func(r *liveRecord) { r.Cases[caseControl] = liveCase{outcomeAllowed, targetAbsent} },
		"names no binary digest": func(r *liveRecord) { r.Binary = "short" },
	} {
		record := validCodexLiveRecord()
		mutate(&record)
		rows := s.m4LiveRowsFrom(t.Context(), writeRecord(t, record), false)
		if rows[0].status != statusFail || !rows[0].required || !strings.Contains(rows[0].detail, reason) {
			t.Errorf("%s: %+v", reason, rows)
		}
	}
}

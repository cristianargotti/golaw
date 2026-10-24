package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// simulatedDigest is the digest of what the simulated build writes.
func simulatedDigest() string {
	sum := sha256.Sum256([]byte("simulated executable"))
	return hex.EncodeToString(sum[:])
}

// validRecord is a complete live record naming the simulated candidate.
func validRecord() liveRecord {
	return liveRecord{
		Date: "2026-10-16", Agent: agentClaude, OS: liveOS, Arch: liveArch,
		AgentVersion: "2.1.299 (Claude Code)", Binary: simulatedDigest(),
		Owner: "the kit owner", Environment: "a personal macOS workstation, isolated home",
		CredentialBoundary: "a subscription login, no repository secrets", SpendingLimit: "1 USD",
		Schedule: "before each tag", Retention: "kept in the repository",
		Cases: map[string]liveCase{
			caseControl:     {outcomeAllowed, targetWritten},
			caseDenied:      {outcomeBlocked, targetUnchanged},
			caseHostTimeout: {outcomeBlocked, targetUnchanged},
			caseHostMissing: {outcomeBlocked, targetUnchanged},
		},
	}
}

// writeRecord writes a record and returns its path.
func writeRecord(t *testing.T, record liveRecord) string {
	t.Helper()
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "claude-darwin.json")
	checkedWrite(t, path, string(data))
	return path
}

func TestLiveRowsPassOnlyForTheCandidate(t *testing.T) {
	t.Parallel()
	s := testSuite(t)
	s.root = t.TempDir() // a tree without the module's own record: the absent case
	for _, required := range []bool{false, true} {
		rows := s.liveRows(t.Context(), required)
		if len(rows) != 2 || rows[0].status != statusSkip || rows[0].required != required ||
			!strings.Contains(rows[1].detail, liveRecordPath) {
			t.Fatalf("%+v", rows)
		}
	}
	rows := s.liveRowsFrom(t.Context(), writeRecord(t, validRecord()), true)
	if rows[0].status != statusPass || rows[1].name != caseHostMissing || !strings.Contains(rows[0].detail, "2026-10-16") {
		t.Fatalf("%+v", rows)
	}
	other := validRecord()
	other.Binary = strings.Repeat("0", len(other.Binary))
	rows = s.liveRowsFrom(t.Context(), writeRecord(t, other), false)
	if rows[0].status != statusSkip || rows[0].required || !strings.Contains(rows[0].detail, simulatedDigest()) {
		t.Fatalf("%+v", rows)
	}
	s.execute = func(context.Context, command) result { return result{err: ErrAcceptance} }
	if rows = s.liveRowsFrom(t.Context(), writeRecord(t, validRecord()), false); rows[0].status != statusFail {
		t.Fatalf("%+v", rows)
	}
}

// brokenRecords are the records the harness refuses, by the reason it names.
func brokenRecords() map[string]func(*liveRecord) {
	return map[string]func(*liveRecord){
		"the live record date":       func(r *liveRecord) { r.Date = "16/10/2026" },
		"not a Claude Code session":  func(r *liveRecord) { r.OS = "linux" },
		"below 2.1.295":              func(r *liveRecord) { r.AgentVersion = "2.1.294" },
		"misses a fact":              func(r *liveRecord) { r.SpendingLimit = " " },
		"names no binary digest":     func(r *liveRecord) { r.Binary = "short" },
		"shows host-timeout allowed": func(r *liveRecord) { r.Cases[caseHostTimeout] = liveCase{outcomeAllowed, targetUnchanged} },
		"holds 3 cases":              func(r *liveRecord) { delete(r.Cases, caseDenied) },
		"shows allowed-control blocked": func(r *liveRecord) {
			r.Cases[caseControl] = liveCase{outcomeBlocked, targetUnchanged}
		},
	}
}

func TestLiveRecordsThatShowAFailureFail(t *testing.T) {
	t.Parallel()
	s := testSuite(t)
	for reason, mutate := range brokenRecords() {
		record := validRecord()
		mutate(&record)
		rows := s.liveRowsFrom(t.Context(), writeRecord(t, record), false)
		if rows[0].status != statusFail || !rows[0].required || !strings.Contains(rows[0].detail, reason) {
			t.Errorf("%s: %+v", reason, rows)
		}
	}
	for name, body := range map[string]string{"undecodable": "{", "unknown key": `{"extra":1}`} {
		path := filepath.Join(t.TempDir(), "record.json")
		checkedWrite(t, path, body)
		if rows := s.liveRowsFrom(t.Context(), path, false); rows[0].status != statusFail ||
			!strings.Contains(rows[0].detail, "decode the live record") {
			t.Errorf("%s: %+v", name, rows)
		}
	}
}

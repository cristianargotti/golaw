package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

// The live record of M1 (section 10.4): one dated session of the pinned
// host on macOS that a person ran with the candidate binary. It lives in
// the repository so the release run reads it, and the binary it names is
// the reproducible darwin/arm64 build of the source, which a commit that
// only adds the record leaves unchanged.
const (
	categoryLive    = "live"
	caseHostTimeout = "host-timeout"
	caseHostMissing = "host-missing-binary"
	caseControl     = "allowed-control"
	caseDenied      = "denied-write"
	liveRecordPath  = "docs/live/claude-darwin.json"
	liveOS          = "darwin"
	liveArch        = "arm64"
	outcomeAllowed  = "allowed"
	outcomeBlocked  = "blocked"
	targetWritten   = "written"
	targetUnchanged = "unchanged"
)

var digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// liveCase is what the session observed for one case.
type liveCase struct {
	Outcome string `json:"outcome"`
	Target  string `json:"target"`
}

// liveRecord is the dated record of one live session with the facts
// section 10.4 asks every record to keep.
type liveRecord struct {
	Date               string              `json:"date"`
	Agent              string              `json:"agent"`
	OS                 string              `json:"os"`
	Arch               string              `json:"arch"`
	AgentVersion       string              `json:"agent_version"`
	Binary             string              `json:"binary_sha256"`
	Owner              string              `json:"owner"`
	Environment        string              `json:"environment"`
	CredentialBoundary string              `json:"credential_boundary"`
	SpendingLimit      string              `json:"spending_limit"`
	Schedule           string              `json:"schedule"`
	Retention          string              `json:"retention"`
	Cases              map[string]liveCase `json:"cases"`
}

func (s *suite) liveRows(ctx context.Context, required bool) []row {
	return s.liveRowsFrom(ctx, filepath.Join(s.root, filepath.FromSlash(liveRecordPath)), required)
}

// liveRowsFrom judges the record at path. An absent record, or one that
// names another binary, is SKIP, which the release run refuses; a record
// that misses a fact or shows a guard failing is FAIL.
func (s *suite) liveRowsFrom(ctx context.Context, path string, required bool) []row {
	record, err := readLiveRecord(path)
	if errors.Is(err, fs.ErrNotExist) {
		return liveRowsWith(statusSkip, "no dated live record at "+liveRecordPath, required)
	}
	if err != nil {
		return liveRowsWith(statusFail, err.Error(), true)
	}
	digest, err := s.candidateDigest(ctx)
	if err != nil {
		return liveRowsWith(statusFail, err.Error(), true)
	}
	if record.Binary != digest {
		return liveRowsWith(statusSkip, fmt.Sprintf(
			"the live record of %s names binary %s, the darwin/arm64 candidate is %s",
			record.Date, record.Binary, digest), required)
	}
	return liveRowsWith(statusPass, fmt.Sprintf("live record of %s by %s on Claude Code %s",
		record.Date, record.Owner, record.AgentVersion), true)
}

func liveRowsWith(status, detail string, required bool) []row {
	return []row{
		{categoryLive, caseHostTimeout, status, detail, required},
		{categoryLive, caseHostMissing, status, detail, required},
	}
}

func readLiveRecord(path string) (liveRecord, error) {
	record, err := decodeLiveRecord(path)
	if err != nil {
		return liveRecord{}, err
	}
	return record, validLiveRecord(record)
}

// decodeLiveRecord reads a live record strictly, whatever agent it names.
func decodeLiveRecord(path string) (liveRecord, error) {
	body, err := os.ReadFile(path) //nolint:gosec // The record path is fixed under the module root or a test directory.
	if err != nil {
		return liveRecord{}, fmt.Errorf("read the live record: %w", err)
	}
	var record liveRecord
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil {
		return liveRecord{}, fmt.Errorf("decode the live record: %w", err)
	}
	return record, nil
}

func validLiveRecord(record liveRecord) error {
	if record.Agent != agentClaude || record.OS != liveOS || record.Arch != liveArch {
		return fmt.Errorf("the live record is not a Claude Code session on darwin/arm64: %w", ErrAcceptance)
	}
	if err := versionFloor(record.AgentVersion); err != nil {
		return err
	}
	if err := recordFacts(record); err != nil {
		return err
	}
	return liveCases(record.Cases)
}

// recordFacts holds a record to what every live record keeps (section
// 10.4): its date, the facts of the person who ran it and the digest of
// the binary it ran.
func recordFacts(record liveRecord) error {
	if _, err := time.Parse(time.DateOnly, record.Date); err != nil {
		return fmt.Errorf("the live record date: %w", err)
	}
	facts := []string{
		record.Owner, record.Environment, record.CredentialBoundary,
		record.SpendingLimit, record.Schedule, record.Retention,
	}
	if slices.ContainsFunc(facts, func(fact string) bool { return strings.TrimSpace(fact) == "" }) {
		return fmt.Errorf("the live record misses a fact of section 10.4: %w", ErrAcceptance)
	}
	if !digestPattern.MatchString(record.Binary) {
		return fmt.Errorf("the live record names no binary digest: %w", ErrAcceptance)
	}
	return nil
}

// liveCases holds the record to the four cases of the M1 live session: the
// control is written, the denial and both host failures leave the target
// unchanged.
func liveCases(cases map[string]liveCase) error {
	want := map[string]liveCase{
		caseControl:     {outcomeAllowed, targetWritten},
		caseDenied:      {outcomeBlocked, targetUnchanged},
		caseHostTimeout: {outcomeBlocked, targetUnchanged},
		caseHostMissing: {outcomeBlocked, targetUnchanged},
	}
	if len(cases) != len(want) {
		return fmt.Errorf("the live record holds %d cases, not the four of M1: %w", len(cases), ErrAcceptance)
	}
	for name, expected := range want {
		if got := cases[name]; got != expected {
			return fmt.Errorf("the live record shows %s %s with the target %s: %w",
				name, got.Outcome, got.Target, ErrAcceptance)
		}
	}
	return nil
}

// candidateDigest is the digest of the reproducible darwin/arm64 build of
// the source, the binary a live record names.
func (s *suite) candidateDigest(ctx context.Context) (string, error) {
	out := filepath.Join(s.base, "candidate", binaryName)
	if err := os.MkdirAll(filepath.Dir(out), dirMode); err != nil {
		return "", fmt.Errorf("create the candidate directory: %w", err)
	}
	env := append(slices.Clone(s.env), "GOOS="+liveOS, "GOARCH="+liveArch, "CGO_ENABLED=0")
	got := s.execute(ctx, command{
		program: s.goTool,
		args:    []string{goBuild, "-trimpath", "-buildvcs=false", "-o", out, "./cmd/golaw"},
		dir:     s.root, env: env,
	})
	if err := success(got); err != nil {
		return "", fmt.Errorf("build the darwin/arm64 candidate: %w", err)
	}
	body, err := readFile(out)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

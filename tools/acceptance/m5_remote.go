package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// The remote gate row, the record of a demonstrated rejection and the page
// that states the generic limitation (A18).
const (
	m5RemoteCategory = "m5-remote"
	m5RemoteRecord   = "docs/live/remote-gate.json"
	m5ProfilesPage   = "docs/profiles.md"
)

// m5Statement is the generic limitation of A18, word for word.
func m5Statement() []string {
	return []string{
		"A profile's remote gate is whatever the adopting platform runs.",
		"Without a remote stage, automatic kit checks run only on machines whose developers armed them.",
	}
}

// m5RemoteGate is the record of one demonstrated remote rejection, the
// platform named in neutral words.
type m5RemoteGate struct {
	Date      string `json:"date"`
	Owner     string `json:"owner"`
	Platform  string `json:"platform"`
	Revision  string `json:"revision"`
	Violation string `json:"violation"`
	Rejection string `json:"rejection"`
}

func m5RemoteCases() []m5Case {
	return []m5Case{{
		name: "gate",
		run: func(s *suite, _ context.Context) error {
			_, err := m5RemoteOutcome(s.root)
			return err
		},
		detail: func(root string) string {
			outcome, err := m5RemoteOutcome(root)
			if err != nil {
				return err.Error()
			}
			return outcome
		},
	}}
}

// m5ReadRemote reads a record strictly: every field present, the date a
// calendar day, the revision a full commit.
func m5ReadRemote(path string) (m5RemoteGate, error) {
	body, err := readFile(path)
	if err != nil {
		return m5RemoteGate{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var record m5RemoteGate
	if stepErr := decoder.Decode(&record); stepErr != nil {
		return m5RemoteGate{}, fmt.Errorf("decode %s: %w: %w", path, ErrAcceptance, stepErr)
	}
	fields := []string{record.Date, record.Owner, record.Platform, record.Revision, record.Violation, record.Rejection}
	for _, field := range fields {
		if strings.TrimSpace(field) == "" {
			return m5RemoteGate{}, fmt.Errorf("%s leaves a field empty: %w", path, ErrAcceptance)
		}
	}
	if _, stepErr := time.Parse(time.DateOnly, record.Date); stepErr != nil {
		return m5RemoteGate{}, fmt.Errorf("%s dates the rejection %q: %w", path, record.Date, ErrAcceptance)
	}
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(record.Revision) {
		return m5RemoteGate{}, fmt.Errorf("%s names the revision %q: %w", path, record.Revision, ErrAcceptance)
	}
	return record, nil
}

// m5RemoteOutcome answers which outcome of A18 holds: a recorded remote
// rejection, or the generic limitation stated; neither is an error.
func m5RemoteOutcome(root string) (string, error) {
	path := filepath.Join(root, filepath.FromSlash(m5RemoteRecord))
	_, statErr := os.Stat(path)
	switch {
	case statErr == nil:
		return m5RecordedRejection(path)
	case !errors.Is(statErr, fs.ErrNotExist):
		return "", fmt.Errorf("read %s: %w", path, statErr)
	}
	page, err := readFile(filepath.Join(root, filepath.FromSlash(m5ProfilesPage)))
	if err != nil {
		return "", err
	}
	for _, sentence := range m5Statement() {
		if !bytes.Contains(page, []byte(sentence)) {
			return "", fmt.Errorf("no remote rejection is recorded and %s does not state %q: %w", m5ProfilesPage, sentence, ErrAcceptance)
		}
	}
	return "no remote rejection is recorded; docs/profiles.md states the generic limitation of A18", nil
}

// m5RecordedRejection is the outcome a valid record names.
func m5RecordedRejection(path string) (string, error) {
	record, err := m5ReadRemote(path)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("remote rejection recorded on %s by %s: %s at %s rejected %s",
		record.Date, record.Owner, record.Platform, record.Revision, record.Violation), nil
}

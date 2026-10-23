package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/hook/agents"
	"github.com/cristianargotti/golaw/internal/install"
)

// The m4-trust category (A20, D14) and what it reads.
const (
	m4TrustCategory     = "m4-trust"
	m4ChangedDefinition = "changed-definition"
	lineTrusted         = "doctor.trusted"
	codexConfig         = "config.toml"
	changedTimeout      = 11
	commandDoctor       = "doctor"
)

// m4TrustRows is the m4-trust category.
func (s *suite) m4TrustRows(ctx context.Context, _ bool) []row {
	return []row{measuredM4(m4TrustCategory, m4ChangedDefinition, s.m4ChangedDefinition(ctx))}
}

// m4ChangedDefinition installs the Codex hook in a home of its own,
// trusts every installed handler as the /hooks review records it, and
// changes the timeout of golaw's PreToolUse handler after the trust:
// doctor reports trusted before the change and revalidation required after
// it, never trusted (A20).
func (s *suite) m4ChangedDefinition(ctx context.Context) error {
	home, env, err := s.m4Home(m4ChangedDefinition)
	if err != nil {
		return err
	}
	t := m4Target{home: home, binary: s.binary, env: env}
	if t.repo, err = s.m4Repository(ctx, t, "m4-"+m4ChangedDefinition); err != nil {
		return err
	}
	steps := []func() error{
		func() error {
			return success(s.execute(ctx, command{program: t.binary, args: []string{commandAgent, verbInstall, flagAgent, agentCodex}, dir: t.repo, env: t.env}))
		},
		func() error { return s.m4Trust(t) },
		func() error { return s.m4TrustState(ctx, t, "ok", "trusted") },
		func() error { return s.m4ChangeTimeout(t) },
		func() error { return s.m4TrustState(ctx, t, "red", "revalidation-required") },
	}
	for i, step := range steps {
		if err := step(); err != nil {
			return fmt.Errorf("step %d of the changed definition: %w", i+1, err)
		}
	}
	return nil
}

// m4GolawEntries are the handlers of an agent's file whose command names
// the binary.
func m4GolawEntries(entries []m4Entry, agent, binary, goos string) []m4Entry {
	return slices.DeleteFunc(slices.Clone(entries), func(entry m4Entry) bool {
		return !m4Names(m4Command(agent, goos, entry.Handler), binary)
	})
}

// m4TrustRecord is the [hooks.state] table of the handlers, the record the
// /hooks review writes into config.toml: the key of each handler's place
// in the hooks file and the hash of its definition (survey section 6). A
// JSON string is a TOML basic string for every key and hash written here.
func m4TrustRecord(hooksPath string, entries []m4Entry, goos string) (string, error) {
	if len(entries) == 0 {
		return "", fmt.Errorf("%s holds no golaw handler to trust: %w", hooksPath, ErrAcceptance)
	}
	var record strings.Builder
	for i := range entries {
		entry := &entries[i]
		hash, err := install.CodexTrustHash(entry.Event, entry.Matcher, entry.Raw, goos)
		if err != nil {
			return "", fmt.Errorf("hash the %s handler: %w", entry.Event, err)
		}
		key, err := json.Marshal(install.CodexTrustKey(hooksPath, entry.Event, entry.Group, entry.Index))
		if err != nil {
			return "", fmt.Errorf("encode the trust key: %w", err)
		}
		value, err := json.Marshal(hash)
		if err != nil {
			return "", fmt.Errorf("encode the trust hash: %w", err)
		}
		fmt.Fprintf(&record, "[hooks.state.%s]\ntrusted_hash = %s\n\n", key, value)
	}
	return record.String(), nil
}

// m4Trust writes the trust record of golaw's installed Codex handlers into
// the Codex home of the target, never the person's.
func (s *suite) m4Trust(t m4Target) error {
	path := m4HooksPath(t.home, agentCodex)
	entries, err := m4ReadEntries(path, agentCodex)
	if err != nil {
		return err
	}
	record, err := m4TrustRecord(path, m4GolawEntries(entries, agentCodex, t.binary, s.m4System()), s.m4System())
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(t.home, codexDir, codexConfig), []byte(record), fileMode)
}

// m4TrustState requires the trust line of the Codex report to read state
// and code.
func (s *suite) m4TrustState(ctx context.Context, t m4Target, state, code string) error {
	line, err := s.m4CodexLine(ctx, t, lineTrusted)
	if err != nil {
		return err
	}
	if line.State != state || line.Code != code {
		return fmt.Errorf("doctor.trusted %s is %s %s (%s), want %s %s: %w", line.Key, line.State, line.Code, line.Detail, state, code, ErrAcceptance)
	}
	return nil
}

// m4CodexLine runs doctor --json in the target and answers the first line
// of an id in the Codex report.
func (s *suite) m4CodexLine(ctx context.Context, t m4Target, id string) (doctor.Line, error) {
	got := s.execute(ctx, command{program: t.binary, args: []string{commandDoctor, m2JSONFlag}, dir: t.repo, env: t.env})
	if err := success(got); err != nil {
		return doctor.Line{}, err
	}
	reports, err := decodeReports(got.stdout)
	if err != nil {
		return doctor.Line{}, err
	}
	index := slices.Index(agents.IDs(), agentCodex)
	if index < 0 || index >= len(reports) {
		return doctor.Line{}, fmt.Errorf("doctor --json printed no Codex report: %w", ErrAcceptance)
	}
	for _, line := range reports[index].Lines {
		if line.ID == id {
			return line, nil
		}
	}
	return doctor.Line{}, fmt.Errorf("the Codex report lacks %s: %w", id, ErrAcceptance)
}

// m4ChangeTimeout gives golaw's PreToolUse handler the timeout 11, a change
// of its definition after the trust (survey section 6.1).
func (s *suite) m4ChangeTimeout(t m4Target) error {
	path := m4HooksPath(t.home, agentCodex)
	entry, err := s.m4Golaw(t, agentCodex, eventPre, toolBash)
	if err != nil {
		return err
	}
	body, err := readFile(path)
	if err != nil {
		return err
	}
	var document struct {
		Hooks map[string][]struct {
			Matcher string           `json:"matcher,omitempty"`
			Hooks   []map[string]any `json:"hooks"`
		} `json:"hooks"`
	}
	if decodeErr := json.Unmarshal(body, &document); decodeErr != nil {
		return fmt.Errorf(decodeFormat, path, decodeErr)
	}
	document.Hooks[eventPre][entry.Group].Hooks[entry.Index]["timeout"] = changedTimeout
	out, err := json.Marshal(document)
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	return writeFile(path, out, fileMode)
}

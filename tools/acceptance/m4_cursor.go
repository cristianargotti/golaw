package main

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/agents"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/policy"
)

// The m4-cursor category: the corpus of the survey and the proof manifest
// of the Cursor adapter, replayed through the installed Cursor command,
// and the key the undocumented Write payload of the corpus carries.
const (
	m4CursorCategory = "m4-cursor"
	m4CursorCorpus   = "internal/hook/cursor/testdata/payloads"
	m4CursorManifest = "internal/hook/cursor/proof/manifest.yml"
	m4UnknownKey     = "__undocumented__"
)

// m4CursorCases are the six payloads of the survey (D19): the shell and
// the write of preToolUse allowed, the write captured before the allow
// (A4), the read of .env refused by name, the commit allowed, and both
// post events quiet over the clean files their commands wrote.
func m4CursorCases() map[string]m4CorpusCase {
	return map[string]m4CorpusCase{
		"pre-tool-use-shell":          {want: m4Allow},
		"pre-tool-use-write-unknown":  {want: m4Allow, check: (*suite).m4Captured},
		"before-read-file":            {want: m4Deny},
		"before-shell-execution":      {want: m4Allow},
		"post-tool-use-shell":         {want: m4Allow, pre: true},
		"post-tool-use-failure-shell": {want: m4Allow, pre: true, written: map[string]string{"out.txt": "x"}},
	}
}

// m4CursorRows is the m4-cursor category.
func (s *suite) m4CursorRows(ctx context.Context, _ bool) []row {
	return s.m4AdapterRows(ctx, m4Adapter{
		agent: agentCursor, category: m4CursorCategory, corpus: m4CursorCorpus,
		manifest: m4CursorManifest, cases: m4CursorCases(),
	})
}

// m4Captured requires the capture record of A4 for the repository: the key
// names of the undocumented Write payload, recorded by the hook process
// before its allow answered, where doctor reads them.
func (s *suite) m4Captured(repo string) error {
	dirs, err := policy.DefaultDirs(s.m4System(), func(name string) string { return environmentValue(s.env, name) })
	if err != nil {
		return fmt.Errorf("resolve the state directory: %w", err)
	}
	_, identity, err := install.ProjectIdentity(repo)
	if err != nil {
		return fmt.Errorf("identify %s: %w", repo, err)
	}
	path := agents.CapturePath(dirs.State, agentCursor, identity)
	body, err := readFile(path)
	if err != nil {
		return err
	}
	var record doctor.CaptureRecord
	if err := json.Unmarshal(body, &record); err != nil {
		return fmt.Errorf(decodeFormat, path, err)
	}
	if !slices.ContainsFunc(record.Captures, func(c hook.Capture) bool {
		return c.Event == cursorPreTool && c.Tool == "Write" && slices.Equal(c.Keys, []string{m4UnknownKey})
	}) {
		return fmt.Errorf("the capture record %s lacks the keys of the Write payload: %w", path, ErrAcceptance)
	}
	return nil
}

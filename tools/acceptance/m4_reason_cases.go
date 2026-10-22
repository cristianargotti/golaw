package main

import (
	"context"
	"path/filepath"
	"strings"
)

// m4DashWrite is a typographic dash written to note.txt. Claude's Write and
// Codex's apply_patch Add File refuse it before the disk, one normalized
// write event; Cursor's writes are captured and allowed (A4), so its reason
// is the context of the postToolUse Write the detection pass judges after
// the write lands.
func (s *suite) m4DashWrite(ctx context.Context) error {
	repo, err := s.repository(ctx, "m4-reason-dash-write", true)
	if err != nil {
		return err
	}
	claude, err := s.m4ClaudeAnswer(ctx, repo, writeFixture("m4-dash-write", pathNote, textDash, wantDeny))
	if err != nil {
		return err
	}
	patch := "*** Begin Patch\n*** Add File: " + pathNote + "\n+" + strings.TrimSuffix(textDash, "\n") + "\n*** End Patch"
	body, err := s.m4Sample(agentCodex, "pre-apply-patch-add.json", repo, map[string]any{"tool_input": map[string]any{keyCommand: patch}})
	if err != nil {
		return err
	}
	codexAnswer, err := s.m4Reason(ctx, s.m4Shared(repo), agentCodex, m4Deny, body)
	if err != nil {
		return err
	}
	cursorReason, err := s.m4CursorWrite(ctx, repo)
	if err != nil {
		return err
	}
	return m4SameText(m4Reasons, map[string]string{
		agentClaude: claude.Hook.Reason, agentCodex: codexAnswer.Reason, agentCursor: cursorReason,
	})
}

// m4CursorWrite sends the Write Cursor captures and allows, writes the dash
// the tool writes, and reads the context of the postToolUse Write.
func (s *suite) m4CursorWrite(ctx context.Context, repo string) (string, error) {
	const file = "pre-tool-use-write-unknown.json"
	pre, err := s.m4Sample(agentCursor, file, repo, nil)
	if err != nil {
		return "", err
	}
	if allowErr := s.m4Allowed(ctx, s.m4Shared(repo), agentCursor, pre); allowErr != nil {
		return "", allowErr
	}
	if writeErr := writeFile(filepath.Join(repo, pathNote), []byte(textDash), fileMode); writeErr != nil {
		return "", writeErr
	}
	post, err := s.m4Sample(agentCursor, file, repo, map[string]any{
		"hook_event_name": cursorPostTool, "tool_output": "{}", "duration": 1,
	})
	if err != nil {
		return "", err
	}
	answer, err := s.m4Reason(ctx, s.m4Shared(repo), agentCursor, m4Block, post)
	return answer.Reason, err
}

// m4SecretRead is the read of .env. As a shell command the three agents
// refuse it before it runs and their English reasons are compared. As a
// read, Codex has no read event (section 8.4) and the beforeReadFile answer
// of Cursor carries no agent message (D5), so the human lines of Claude's
// Read and Cursor's beforeReadFile, one normalized read event, are
// compared.
func (s *suite) m4SecretRead(ctx context.Context) error {
	repo, err := s.repository(ctx, "m4-reason-secret-read", true)
	if err != nil {
		return err
	}
	reasons, err := s.m4ShellReasons(ctx, repo, "m4-secret-read", m4ReadCommand)
	if err != nil {
		return err
	}
	if sameErr := m4SameText(m4Reasons, reasons); sameErr != nil {
		return sameErr
	}
	claude, err := s.m4ClaudeAnswer(ctx, repo, fixture{
		name: "m4-secret-file", category: m4ReasonCategory, event: eventPre, tool: toolRead, want: wantDeny,
		input: map[string]any{keyFilePath: m4DotenvFile},
	})
	if err != nil {
		return err
	}
	body, err := s.m4Sample(agentCursor, "before-read-file.json", repo, nil)
	if err != nil {
		return err
	}
	cursorAnswer, err := s.m4Reason(ctx, s.m4Shared(repo), agentCursor, m4Deny, body)
	if err != nil {
		return err
	}
	return m4SameText("human lines", map[string]string{agentClaude: claude.Human, agentCursor: cursorAnswer.Human})
}

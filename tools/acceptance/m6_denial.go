package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/surfacefix"
)

// m6Patch is the apply_patch command Codex sends to write after at path:
// an Add File for a new file, an Update File that replaces every line of
// before otherwise, so the guard reads the whole candidate either way.
func m6Patch(path string, before, after []byte, existed bool) string {
	lines := []string{"*** Begin Patch", "*** Add File: " + path}
	if existed {
		lines = append(lines[:1], "*** Update File: "+path, "@@")
		lines = append(lines, m6Lines("-", before)...)
	}
	lines = append(lines, m6Lines("+", after)...)
	return strings.Join(append(lines, "*** End Patch"), "\n")
}

// m6Lines are the lines of text, each behind mark.
func m6Lines(mark string, text []byte) []string {
	lines := strings.Split(strings.TrimSuffix(string(text), "\n"), "\n")
	for i := range lines {
		lines[i] = mark + lines[i]
	}
	return lines
}

// The fields of the corpus payloads a row replaces.
const (
	m6ToolInput  = "tool_input"
	m6ToolOutput = "tool_output"
	m6Duration   = "duration"
)

// m6Wants are the answers a red write and its near miss must take.
func m6Wants(red bool) (string, m4Want) {
	if red {
		return wantDeny, m4Deny
	}
	return wantAllow, m4Allow
}

// m6ClaudeWrite is Claude's Write of content before it lands: the reason
// of a red write, nothing for a near miss.
func (s *m6Suite) m6ClaudeWrite(ctx context.Context, repo, path string, content []byte, red bool) (string, error) {
	got, err := s.m6Claude(ctx, repo, eventPre, path, content, s.env)
	if err != nil {
		return "", err
	}
	want, _ := m6Wants(red)
	if err := expect(got, want); err != nil {
		return "", fmt.Errorf("%s through Claude: %w", path, err)
	}
	if !red {
		return "", nil
	}
	var decoded answer
	if err := json.Unmarshal([]byte(got.stdout), &decoded); err != nil {
		return "", fmt.Errorf("decode the Claude answer: %w", err)
	}
	return decoded.Hook.Reason, nil
}

// m6Before is the file at path in the checkout and whether it exists.
func m6Before(repo, path string) ([]byte, bool, error) {
	full := filepath.Join(repo, filepath.FromSlash(path))
	if _, err := os.Stat(full); errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	body, err := readFile(full)
	return body, err == nil, err
}

// m6CodexWrite is Codex's apply_patch of content before it lands.
func (s *m6Suite) m6CodexWrite(ctx context.Context, repo, path string, content []byte, red bool) (string, error) {
	before, existed, err := m6Before(repo, path)
	if err != nil {
		return "", err
	}
	patch := m6Patch(path, before, content, existed)
	body, err := s.m4Sample(agentCodex, "pre-apply-patch-add.json", repo, map[string]any{m6ToolInput: map[string]any{keyCommand: patch}})
	if err != nil {
		return "", err
	}
	got := s.execute(ctx, command{program: s.binary, args: []string{commandHook, agentCodex}, dir: repo, env: s.env, input: string(body)})
	_, want := m6Wants(red)
	reply, err := judgeCodex(eventPre, want, got)
	if err != nil {
		return "", fmt.Errorf("%s through Codex: %w", path, err)
	}
	return reply.Reason, nil
}

// m6CursorWrite is Cursor's captured write: allowed before it runs, the
// bytes written, then judged after it ran, the order of m4CursorWrite.
func (s *m6Suite) m6CursorWrite(ctx context.Context, repo, path string, content []byte, red bool) (string, error) {
	const file = "pre-tool-use-write-unknown.json"
	run := func(event string, want m4Want, set map[string]any) (m4Answer, error) {
		body, err := s.m4Sample(agentCursor, file, repo, set)
		if err != nil {
			return m4Answer{}, err
		}
		got := s.execute(ctx, command{program: s.binary, args: []string{commandHook, agentCursor}, dir: repo, env: s.env, input: string(body)})
		reply, err := judgeCursor(event, want, got)
		if err != nil {
			return m4Answer{}, fmt.Errorf("%s through Cursor %s: %w", path, event, err)
		}
		return reply, nil
	}
	if _, err := run(cursorPreTool, m4Allow, nil); err != nil {
		return "", err
	}
	if err := writeFile(filepath.Join(repo, filepath.FromSlash(path)), content, fileMode); err != nil {
		return "", err
	}
	want := m4Allow
	if red {
		want = m4Block
	}
	reply, err := run(cursorPostTool, want, map[string]any{"hook_event_name": cursorPostTool, m6ToolOutput: "{}", m6Duration: 1})
	return reply.Reason, err
}

// m6Agents writes content at path through Claude, Codex and Cursor, keyed
// by agent; the two that answer before the write run first, so the
// checkout holds the bytes only from Cursor's write on.
func (s *m6Suite) m6Agents(ctx context.Context, repo, path string, content []byte, red, cursor bool) (map[string]string, error) {
	writes := map[string]func(context.Context, string, string, []byte, bool) (string, error){
		agentClaude: s.m6ClaudeWrite, agentCodex: s.m6CodexWrite, agentCursor: s.m6CursorWrite,
	}
	agents := []string{agentClaude, agentCodex}
	if cursor {
		agents = append(agents, agentCursor)
	}
	reasons := make(map[string]string, len(agents))
	for _, agent := range agents {
		reason, err := writes[agent](ctx, repo, path, content, red)
		if err != nil {
			return nil, err
		}
		reasons[agent] = reason
	}
	return reasons, nil
}

// m6Deny proves one certain check: its red write denied on Claude and Codex
// with the same M6.6.5 sentence and fed back to Cursor after the write, its
// near miss allowed everywhere.
func (s *m6Suite) m6Deny(ctx context.Context, d m6Denial) error {
	repo, err := s.m6Twin(ctx, "denial-"+d.name, surfacefix.Corrected, m6WithoutFeedback)
	if err != nil {
		return err
	}
	red, near, err := d.contents(s.root)
	if err != nil {
		return err
	}
	cursor := d.name != "analyzer-widening"
	if redErr := s.m6DenyRed(ctx, repo, d, red, cursor); redErr != nil {
		return redErr
	}
	_, err = s.m6Agents(ctx, repo, d.path, near, false, cursor)
	return err
}

// m6DenyRed writes the red bytes through the agents, judges their reasons
// and puts the file back as it was, so the near miss meets the twin alone.
func (s *m6Suite) m6DenyRed(ctx context.Context, repo string, d m6Denial, red []byte, cursor bool) error {
	before, existed, err := m6Before(repo, d.path)
	if err != nil {
		return err
	}
	reasons, err := s.m6Agents(ctx, repo, d.path, red, true, cursor)
	if err != nil {
		return err
	}
	if err := m6JudgeReasons(d, reasons, cursor); err != nil {
		return err
	}
	return m6Restore(repo, d.path, before, existed)
}

// m6JudgeReasons holds Claude and Codex to one sentence of M6.6.5 and
// Cursor's context to the place and message of that sentence, because
// Cursor reads a certain finding after the write as the feedback line
// "path:line check (Sonar key) message".
func m6JudgeReasons(d m6Denial, reasons map[string]string, cursor bool) error {
	if err := m4SameText(m4Reasons, map[string]string{agentClaude: reasons[agentClaude], agentCodex: reasons[agentCodex]}); err != nil {
		return err
	}
	if !d.reason.MatchString(reasons[agentClaude]) {
		return fmt.Errorf("the reason %q is not the M6.6.5 sentence %s: %w", reasons[agentClaude], d.reason, ErrAcceptance)
	}
	if !cursor {
		return nil
	}
	where, message, _ := strings.Cut(reasons[agentClaude], " ")
	if fed := reasons[agentCursor]; !strings.Contains(fed, where+" ") || !strings.Contains(fed, message) {
		return fmt.Errorf("cursor's feedback %q lacks the reason %q: %w", fed, reasons[agentClaude], ErrAcceptance)
	}
	return nil
}

// m6Restore puts path back as it was before Cursor wrote it.
func m6Restore(repo, path string, before []byte, existed bool) error {
	full := filepath.Join(repo, filepath.FromSlash(path))
	if existed {
		return writeFile(full, before, fileMode)
	}
	if err := os.Remove(full); err != nil {
		return fmt.Errorf("remove the written %s: %w", path, err)
	}
	return nil
}

// m6CorrectedTwin writes every corrected file over the committed red twin
// through Claude: each write is a fix of a red file or a new file of the
// fixes, so a denial here is a false one.
func (s *m6Suite) m6CorrectedTwin(ctx context.Context) error {
	repo, err := s.m6Twin(ctx, "corrected-twin", surfacefix.Red, m6WithoutFeedback)
	if err != nil {
		return err
	}
	files, err := surfacefix.Files(surfacefix.Corrected)
	if err != nil {
		return fmt.Errorf("read the corrected twin: %w", err)
	}
	for _, path := range slices.Sorted(maps.Keys(files)) {
		got, err := s.m6Claude(ctx, repo, eventPre, path, files[path], s.env)
		if err != nil {
			return err
		}
		if err := expect(got, wantAllow); err != nil {
			return fmt.Errorf("the corrected %s was denied: %w", path, err)
		}
	}
	return nil
}

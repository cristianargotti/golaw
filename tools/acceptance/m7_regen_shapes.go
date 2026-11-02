package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// m7ClaudePost replays a Claude post event and answers the context the
// agent reads, empty when the hook printed nothing.
func (s *m7Suite) m7ClaudePost(ctx context.Context, repo string, f fixture) (string, error) {
	body, err := payload(s.root, repo, f)
	if err != nil {
		return "", err
	}
	got := s.execute(ctx, command{program: s.binary, args: []string{commandHook, agentClaude}, dir: repo, env: s.env, input: string(body)})
	if got.err != nil || got.code != 0 || got.stderr != "" {
		return "", fmt.Errorf("the %s post event answered %d %q: %w", f.tool, got.code, strings.TrimSpace(got.stderr), ErrAcceptance)
	}
	if got.stdout == "" {
		return "", nil
	}
	var decoded answer
	if err := json.Unmarshal([]byte(got.stdout), &decoded); err != nil {
		return "", fmt.Errorf(m7DecodeClaude, err)
	}
	return decoded.Hook.Additional, nil
}

// m7CodexPost is Codex's apply_patch of the edit after it landed: the
// lines come back as the reason of the post block (contract 1.5).
func (s *m7Suite) m7CodexPost(ctx context.Context, repo string, before, after []byte) (string, error) {
	set := map[string]any{m6ToolInput: map[string]any{keyCommand: m6Patch(m7PortsPath, before, after, true)}}
	body, err := s.m4Sample(agentCodex, "post-apply-patch.json", repo, set)
	if err != nil {
		return "", err
	}
	got := s.execute(ctx, command{program: s.binary, args: []string{commandHook, agentCodex}, dir: repo, env: s.env, input: string(body)})
	reply, err := judgeCodex(eventPost, m4Block, got)
	if err != nil {
		return "", fmt.Errorf("the apply_patch post event: %w", err)
	}
	return reply.Reason, nil
}

// m7Heredoc is a Bash heredoc over the port: the pre event records the
// state the change detection of the post event compares (M7.9.1), so the
// checkout holds the old bytes until the command runs.
func (s *m7Suite) m7Heredoc(ctx context.Context, repo string, before, after []byte) (string, error) {
	full := filepath.Join(repo, filepath.FromSlash(m7PortsPath))
	if err := writeFile(full, before, fileMode); err != nil {
		return "", err
	}
	f := fixture{name: "m7-regen-heredoc", event: eventPre, tool: toolBash, input: map[string]any{keyCommand: m7HeredocStart + string(after) + "EOF"}}
	if _, err := s.m7Claude(ctx, repo, f, false); err != nil {
		return "", err
	}
	if err := writeFile(full, after, fileMode); err != nil {
		return "", err
	}
	f.event = eventPost
	return s.m7ClaudePost(ctx, repo, f)
}

// m7CursorPost is Cursor's captured write of the edit: its post event finds
// the change against the state its pre event recorded, so the checkout
// holds the old bytes until the write.
func (s *m7Suite) m7CursorPost(ctx context.Context, repo string, before, after []byte) (string, error) {
	if err := writeFile(filepath.Join(repo, filepath.FromSlash(m7PortsPath)), before, fileMode); err != nil {
		return "", err
	}
	return s.m6CursorWrite(ctx, repo, m7PortsPath, after, true)
}

// m7RegenerateOff: with ladder.hook_regenerate off the port edit is allowed
// and nothing regenerates, so the switch costs the developer nothing.
func (s *m7Suite) m7RegenerateOff(ctx context.Context) error {
	repo, err := s.m7Denied(ctx, "regenerate-off")
	if err != nil {
		return err
	}
	if editErr := m6EditPolicy(repo, m7WithoutRegenerate); editErr != nil {
		return editErr
	}
	if commitErr := s.m6Commit(ctx, repo, m7SwitchOff); commitErr != nil {
		return commitErr
	}
	before, err := readFile(filepath.Join(repo, filepath.FromSlash(m7PortsPath)))
	if err != nil {
		return err
	}
	after, err := m7PortsWithPurge(before)
	if err != nil {
		return err
	}
	if _, err := s.m6ClaudeWrite(ctx, repo, m7PortsPath, after, false); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(repo, filepath.FromSlash(m7PortsPath)), after, fileMode); err != nil {
		return err
	}
	return s.m7JudgeOff(ctx, repo, after)
}

// m7JudgeOff runs the post Write with the switch off and refuses any trace
// of a regeneration in the context or in the owned file.
func (s *m7Suite) m7JudgeOff(ctx context.Context, repo string, after []byte) error {
	text, err := s.m7ClaudePost(ctx, repo, fixture{name: "m7-regen-off", event: eventPost, tool: toolWrite, input: map[string]any{
		keyFilePath: filepath.Join(repo, filepath.FromSlash(m7PortsPath)), m5KeyContent: string(after),
	}})
	if err != nil {
		return err
	}
	if strings.Contains(text, m7RegenPrefix) {
		return fmt.Errorf("the hook regenerated with hook_regenerate off: %q: %w", text, ErrAcceptance)
	}
	owned, err := readFile(filepath.Join(repo, filepath.FromSlash(m7OwnedPlans)))
	if err != nil {
		return err
	}
	if strings.Contains(string(owned), m7PurgeHole) {
		return fmt.Errorf("%s gained %s with hook_regenerate off: %w", m7OwnedPlans, m7PurgeHole, ErrAcceptance)
	}
	return nil
}

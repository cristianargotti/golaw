package main

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"
)

// The m4-reason category (D4): one normalized event, the English reason of
// every agent that prints one, byte for byte.
const (
	m4ReasonCategory = "m4-reason"
	m4DotenvFile     = ".env"
	m4ReadCommand    = "cat " + m4DotenvFile
	m4SkipHooks      = "git commit --no-verify -m x"
	m4Reasons        = "reasons"
)

// m4ReasonRows is the m4-reason category.
func (s *suite) m4ReasonRows(ctx context.Context, _ bool) []row {
	return []row{
		measuredM4(m4ReasonCategory, "dash-write", s.m4DashWrite(ctx)),
		measuredM4(m4ReasonCategory, "secret-read", s.m4SecretRead(ctx)),
		measuredM4(m4ReasonCategory, "no-verify", s.m4NoVerify(ctx)),
	}
}

// m4Sample is a payload of an agent's corpus in the repository, with the
// top-level fields of set replaced.
func (s *suite) m4Sample(agent, file, repo string, set map[string]any) ([]byte, error) {
	corpus := m4CodexCorpus
	if agent == agentCursor {
		corpus = m4CursorCorpus
	}
	body, err := readFile(filepath.Join(s.root, filepath.FromSlash(corpus), file))
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if decodeErr := json.Unmarshal(m4Payload(body, repo), &payload); decodeErr != nil {
		return nil, fmt.Errorf(decodeFormat, file, decodeErr)
	}
	maps.Copy(payload, set)
	out, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode the %s sample: %w", agent, err)
	}
	return out, nil
}

// m4Reason replays a payload for an agent and reads the answer of want,
// every shell of the system printing the same one.
func (s *suite) m4Reason(ctx context.Context, t m4Target, agent string, want m4Want, payload []byte) (m4Answer, error) {
	event, tool := m4RouteOf(agent, payload)
	replies, err := s.m4Invoke(ctx, t, agent, event, tool, payload)
	if err != nil {
		return m4Answer{}, err
	}
	judge := judgeCodex
	if agent == agentCursor {
		judge = judgeCursor
	}
	var first m4Answer
	for i, reply := range replies {
		answer, judgeErr := judge(event, want, reply.got)
		if judgeErr != nil {
			return m4Answer{}, fmt.Errorf("%s through %s: %w", agent, reply.shell, judgeErr)
		}
		if i > 0 && answer != first {
			return m4Answer{}, fmt.Errorf("%s answers %+v through %s and %+v through %s: %w", agent, first, replies[0].shell, answer, reply.shell, ErrAcceptance)
		}
		first = answer
	}
	return first, nil
}

// m4ClaudeAnswer sends a fixture through the installed Claude exec form and
// reads its denial.
func (s *suite) m4ClaudeAnswer(ctx context.Context, repo string, f fixture) (answer, error) {
	body, err := payload(s.root, repo, f)
	if err != nil {
		return answer{}, err
	}
	got := s.invoke(ctx, repo, f.event, f.tool, body)
	if err := expect(got, wantDeny); err != nil {
		return answer{}, fmt.Errorf("%s through Claude: %w", f.name, err)
	}
	var decoded answer
	if err := json.Unmarshal([]byte(got.stdout), &decoded); err != nil {
		return answer{}, fmt.Errorf("decode the Claude answer: %w", err)
	}
	return decoded, nil
}

// m4SameText requires the texts of every agent non-empty and equal byte
// for byte, naming each agent's text when they differ.
func m4SameText(what string, texts map[string]string) error {
	agents := slices.Sorted(maps.Keys(texts))
	for _, agent := range agents {
		if texts[agent] == "" || texts[agent] != texts[agents[0]] {
			named := make([]string, 0, len(agents))
			for _, name := range agents {
				named = append(named, fmt.Sprintf("%s %q", name, texts[name]))
			}
			return fmt.Errorf("the %s differ: %s: %w", what, strings.Join(named, ", "), ErrAcceptance)
		}
	}
	return nil
}

// m4ShellSample is the corpus payload of an agent's shell pre event.
func m4ShellSample(agent string) string {
	if agent == agentCursor {
		return "before-shell-execution.json"
	}
	return "pre-bash-plain.json"
}

// m4ShellPayload is the shell pre event of an agent running line in the
// repository.
func (s *suite) m4ShellPayload(agent, repo, line string) ([]byte, error) {
	set := map[string]any{keyCommand: line}
	if agent == agentCodex {
		set = map[string]any{"tool_input": map[string]any{keyCommand: line}}
	}
	return s.m4Sample(agent, m4ShellSample(agent), repo, set)
}

// m4ShellReasons are the reasons the three agents give a shell command
// before it runs: Claude's Bash, Codex's Bash and Cursor's
// beforeShellExecution, one normalized shell event.
func (s *suite) m4ShellReasons(ctx context.Context, repo, name, line string) (map[string]string, error) {
	claude, err := s.m4ClaudeAnswer(ctx, repo, fixture{
		name: name, category: m4ReasonCategory, event: eventPre, tool: toolBash, want: wantDeny,
		input: map[string]any{keyCommand: line},
	})
	if err != nil {
		return nil, err
	}
	reasons := map[string]string{agentClaude: claude.Hook.Reason}
	for _, agent := range []string{agentCodex, agentCursor} {
		body, sampleErr := s.m4ShellPayload(agent, repo, line)
		if sampleErr != nil {
			return nil, sampleErr
		}
		got, reasonErr := s.m4Reason(ctx, s.m4Shared(repo), agent, m4Deny, body)
		if reasonErr != nil {
			return nil, reasonErr
		}
		reasons[agent] = got.Reason
	}
	return reasons, nil
}

// m4NoVerify is the hook bypass on the three agents.
func (s *suite) m4NoVerify(ctx context.Context) error {
	repo, err := s.repository(ctx, "m4-reason-no-verify", true)
	if err != nil {
		return err
	}
	reasons, err := s.m4ShellReasons(ctx, repo, "m4-no-verify", m4SkipHooks)
	if err != nil {
		return err
	}
	return m4SameText(m4Reasons, reasons)
}

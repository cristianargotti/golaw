package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// m4StderrTail is how much of a failed session's stderr a row keeps.
const m4StderrTail = 500

// m4LanePrepare builds the candidate the release gate rebuilds, prepares a
// temporary home with its own CODEX_HOME and an enrolled temporary
// repository, installs the Codex hook with the candidate, writes the trust
// record the /hooks review writes, and requires doctor to report the hook
// trusted, so a session never runs an inert hook.
func (s *suite) m4LanePrepare(ctx context.Context) (m4Target, string, error) {
	digest, err := s.candidateDigest(ctx)
	if err != nil {
		return m4Target{}, "", err
	}
	home, env, err := s.m4Home(m4LaneRepo)
	if err != nil {
		return m4Target{}, "", err
	}
	t := m4Target{home: home, binary: filepath.Join(s.base, m4Candidate, binaryName), env: env}
	if t.repo, err = s.m4Repository(ctx, t, m4LaneRepo); err != nil {
		return m4Target{}, "", err
	}
	steps := []func() error{
		func() error {
			return success(s.execute(ctx, command{program: t.binary, args: []string{commandAgent, verbInstall, flagAgent, agentCodex}, dir: t.repo, env: t.env}))
		},
		func() error { return s.m4Trust(t) },
		func() error { return s.m4TrustState(ctx, t, "ok", "trusted") },
	}
	for i, step := range steps {
		if err := step(); err != nil {
			return m4Target{}, "", fmt.Errorf("step %d of the lane preparation: %w", i+1, err)
		}
	}
	return t, digest, nil
}

// m4LanePrompt pins a session to the apply_patch tool and one file.
func m4LanePrompt(target, line string) string {
	return "Use the apply_patch tool, and no other tool, to add the file " + target +
		" whose only line is: " + line + "\nDo not retry with other content and do not run any command."
}

// m4LaneSession runs one codex exec session in the repository with the
// credential in its environment alone, never on a command line, and
// answers what the session printed, the evidence of what Codex did.
func (s *suite) m4LaneSession(ctx context.Context, t m4Target, codexPath, credential, prompt string) (string, error) {
	env := append(slices.Clone(t.env), m4LaneVariable+"="+credential)
	got := s.execute(ctx, command{
		program: codexPath,
		args:    []string{"exec", "--cd", t.repo, "--sandbox", "workspace-write", "--ephemeral", "--color", "never", prompt},
		dir:     t.repo, env: env, timeout: m4LaneTimeout,
	})
	if got.err != nil {
		return "", fmt.Errorf("run codex exec: %w", got.err)
	}
	if got.code != 0 {
		tail := got.stderr
		if len(tail) > m4StderrTail {
			tail = tail[len(tail)-m4StderrTail:]
		}
		return "", fmt.Errorf("codex exec exited %d: %s: %w", got.code, strings.TrimSpace(tail), ErrAcceptance)
	}
	return got.stdout + got.stderr, nil
}

// m4DeniedCase observes the denied write: blocked with its target absent
// only when the file is absent and the session itself shows Codex blocking
// the patch of that file, so a model that declines on its own is never
// recorded as a blocked write.
func m4DeniedCase(repo, output string) (liveCase, error) {
	_, statErr := os.Stat(filepath.Join(repo, filepath.FromSlash(m4DeniedTarget)))
	switch {
	case statErr == nil:
		return liveCase{outcomeAllowed, targetPresent}, nil
	case !errors.Is(statErr, fs.ErrNotExist):
		return liveCase{}, fmt.Errorf("inspect %s: %w", m4DeniedTarget, statErr)
	case !strings.Contains(output, m4HookBlock) || !strings.Contains(output, m4DeniedTarget):
		return liveCase{}, fmt.Errorf("the session shows no hook denial of %s, so the model may have declined on its own: %w", m4DeniedTarget, ErrAcceptance)
	}
	return liveCase{outcomeBlocked, targetAbsent}, nil
}

// m4LaneSessions runs the denied write, an em dash in a new file, and the
// control, the same line with a hyphen, and observes each: the target
// on disk, the session's own output and, for the denial, the dispatch
// doctor reports, so a model that declines on its own is never recorded
// as a blocked write.
func (s *suite) m4LaneSessions(ctx context.Context, t m4Target, codexPath, credential string) (map[string]liveCase, error) {
	denied := strings.TrimSuffix(textDash, "\n")
	output, err := s.m4LaneSession(ctx, t, codexPath, credential, m4LanePrompt(m4DeniedTarget, denied))
	if err != nil {
		return nil, err
	}
	deniedCase, err := m4DeniedCase(t.repo, output)
	if err != nil {
		return nil, err
	}
	dispatch, err := s.m4CodexLine(ctx, t, lineDispatch)
	if err != nil {
		return nil, err
	}
	if dispatch.Code != "observed" {
		return nil, fmt.Errorf("the hook did not run for the repository: doctor.dispatch is %s: %w", dispatch.Code, ErrAcceptance)
	}
	if _, err := s.m4LaneSession(ctx, t, codexPath, credential, m4LanePrompt(m4ControlTarget, m4ControlLine)); err != nil {
		return nil, err
	}
	body, readErr := readFile(filepath.Join(t.repo, filepath.FromSlash(m4ControlTarget)))
	controlCase := liveCase{outcomeBlocked, targetAbsent}
	if readErr == nil && strings.TrimSpace(string(body)) == m4ControlLine {
		controlCase = liveCase{outcomeAllowed, targetPresent}
	}
	return map[string]liveCase{caseDenied: deniedCase, caseControl: controlCase}, nil
}

package main

import (
	"context"
	"fmt"
	"io/fs"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/proof"
	"github.com/cristianargotti/golaw/internal/surfacefix"
)

// The directory of the M6 fixtures in the module, the message that adopts
// a fixture repository and the prefix of the checks a lint tool answers.
const (
	m6FixtureRoot  = "tools/acceptance/testdata/m6"
	m6AdoptMessage = "feat: adopt the golaw policy over the fixture"
	m6LintPrefix   = "lint."
)

// m6Fixture reads the files of one M6 fixture directory, keyed by slash
// path without the escaped suffix. Every file carries the suffix so no
// linter the kit runs over itself, and no configuration lookup of such a
// tool, ever reads a fixture as the kit's own.
func m6Fixture(root, dir string) (map[string][]byte, error) {
	base := filepath.Join(root, filepath.FromSlash(m6FixtureRoot), filepath.FromSlash(dir))
	files := make(map[string][]byte)
	err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || !entry.Type().IsRegular() {
			return walkErr
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return fmt.Errorf("relativize the M6 fixture file: %w", err)
		}
		name, found := strings.CutSuffix(filepath.ToSlash(rel), proof.EscapedSuffix)
		if !found {
			return fmt.Errorf("the M6 fixture file %s lacks the %s suffix: %w", rel, proof.EscapedSuffix, ErrAcceptance)
		}
		body, err := readFile(path)
		if err != nil {
			return err
		}
		files[name] = proof.Unescape(body)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("read the M6 fixture %s: %w", dir, err)
	}
	return files, nil
}

// m6Edit changes the policy init wrote before the adoption is committed.
type m6Edit func(policy []byte) ([]byte, error)

// m6WithoutFeedback switches the hook feedback off on the ladder line. The
// ladder stays outside the policy digest, so the edit never stales the
// enrollment init recorded.
func m6WithoutFeedback(policy []byte) ([]byte, error) {
	ladder := regexp.MustCompile(`(?m)^ladder: \{(.*) \}$`)
	if !ladder.Match(policy) {
		return nil, fmt.Errorf("the policy holds no ladder line: %w", ErrAcceptance)
	}
	return ladder.ReplaceAll(policy, []byte("ladder: {${1}, hook_feedback: false }")), nil
}

// m6WriteFiles writes files into repo in path order, so a failure always
// names the same file.
func m6WriteFiles(repo string, files map[string][]byte) error {
	for _, path := range slices.Sorted(maps.Keys(files)) {
		if err := writeFile(filepath.Join(repo, filepath.FromSlash(path)), files[path], fileMode); err != nil {
			return err
		}
	}
	return nil
}

// m6Commit stages every change of repo and commits it.
func (s *m6Suite) m6Commit(ctx context.Context, repo, message string) error {
	return s.gitSteps(ctx, repo, []string{gitAdd, "--all"}, commitArgs(message))
}

// m6Repository creates a repository from files, adopts the kit with the
// go and image capabilities of M5's adopted repositories, so the docker
// checks and the go-only coverage command both apply, edits the policy
// when edit is set and commits the adoption.
func (s *m6Suite) m6Repository(ctx context.Context, name string, files map[string][]byte, edit m6Edit) (string, error) {
	repo := filepath.Join(s.base, dirRepos, name)
	if err := m6WriteFiles(repo, files); err != nil {
		return "", err
	}
	if err := s.gitSteps(ctx, repo, []string{commandInit, m2Quiet}); err != nil {
		return "", err
	}
	if err := success(s.m5Golaw(ctx, repo, commandInit, m5CapabilitiesFlag, m5Capabilities, flagApply)); err != nil {
		return "", fmt.Errorf("adopt the fixture repository %s: %w", name, err)
	}
	if edit != nil {
		if err := m6EditPolicy(repo, edit); err != nil {
			return "", err
		}
	}
	if err := s.m6Commit(ctx, repo, m6AdoptMessage); err != nil {
		return "", err
	}
	return repo, nil
}

// m6EditPolicy applies edit to the policy of repo.
func m6EditPolicy(repo string, edit m6Edit) error {
	path := filepath.Join(repo, policyFile)
	policy, err := readFile(path)
	if err != nil {
		return err
	}
	policy, err = edit(policy)
	if err != nil {
		return err
	}
	return writeFile(path, policy, fileMode)
}

// m6Twin is a repository of one twin of the surface fixture, adopted.
func (s *m6Suite) m6Twin(ctx context.Context, name string, twin surfacefix.Twin, edit m6Edit) (string, error) {
	files, err := surfacefix.Files(twin)
	if err != nil {
		return "", fmt.Errorf("read the surface twin: %w", err)
	}
	return s.m6Repository(ctx, name, files, edit)
}

// m6ClaudeCommand is the Claude hook call of a Write of content at path in
// repo; the budget rows reuse it for every timed run.
func (s *m6Suite) m6ClaudeCommand(repo, event, path string, content []byte, env []string) (command, error) {
	body, err := payload(s.root, repo, fixture{
		name: "m6-" + filepath.Base(path), event: event, tool: toolWrite,
		input: map[string]any{keyFilePath: filepath.Join(repo, filepath.FromSlash(path)), m5KeyContent: string(content)},
	})
	if err != nil {
		return command{}, err
	}
	return command{program: s.binary, args: []string{commandHook, agentClaude}, dir: repo, env: env, input: string(body)}, nil
}

// m6Claude runs the Claude hook call of a Write of content at path.
func (s *m6Suite) m6Claude(ctx context.Context, repo, event, path string, content []byte, env []string) (result, error) {
	c, err := s.m6ClaudeCommand(repo, event, path, content, env)
	if err != nil {
		return result{}, err
	}
	return s.execute(ctx, c), nil
}

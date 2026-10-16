package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"slices"
)

type hookHandler struct {
	Type      string   `json:"type"`
	Command   string   `json:"command"`
	Args      []string `json:"args"`
	Timeout   int      `json:"timeout"`
	OnFailure string   `json:"onFailure"`
}

type hookGroup struct {
	Matcher string        `json:"matcher"`
	Hooks   []hookHandler `json:"hooks"`
}

func discoverHooks(home string) (string, map[string][]hookGroup, error) {
	var found string
	var hooks map[string][]hookGroup
	err := filepath.WalkDir(home, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || entry.Name() != hooksFileName {
			return walkErr
		}
		current, err := readHooks(path)
		if err != nil {
			return err
		}
		if !containsGolaw(current) {
			return nil
		}
		if found != "" {
			return fmt.Errorf("multiple installed hook files: %w", ErrAcceptance)
		}
		found, hooks = path, current
		return nil
	})
	if err != nil {
		return "", nil, fmt.Errorf("discover installed hooks: %w", err)
	}
	if found == "" {
		return "", nil, fmt.Errorf("installed hooks absent: %w", ErrAcceptance)
	}
	return found, hooks, nil
}

func readHooks(path string) (map[string][]hookGroup, error) {
	body, err := readFile(path)
	if err != nil {
		return nil, err
	}
	var document struct {
		Hooks map[string][]hookGroup `json:"hooks"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return nil, fmt.Errorf("decode installed hooks: %w", err)
	}
	return document.Hooks, nil
}

func containsGolaw(hooks map[string][]hookGroup) bool {
	for _, groups := range hooks {
		for _, group := range groups {
			for _, handler := range group.Hooks {
				if handler.Command == binaryName {
					return true
				}
			}
		}
	}
	return false
}

func (s *suite) invoke(ctx context.Context, dir, event, tool string, input []byte) result {
	return s.invokeIn(ctx, s.env, dir, event, tool, input)
}

// invokeIn runs the installed handler of an event with an environment of
// its own, the home of a row that installs apart.
func (s *suite) invokeIn(ctx context.Context, env []string, dir, event, tool string, input []byte) result {
	handler, err := s.handler(event, tool)
	if err != nil {
		return result{err: err}
	}
	binary, err := resolveExecutable(handler.Command, env)
	if err != nil {
		return result{err: err}
	}
	if filepath.Clean(binary) != filepath.Clean(s.binary) {
		return result{err: fmt.Errorf("installed command resolves to another binary: %w", ErrAcceptance)}
	}
	return s.execute(ctx, command{
		program: binary, argv0: handler.Command, args: handler.Args,
		dir: dir, env: env, input: string(input),
	})
}

func (s *suite) handler(event, tool string) (hookHandler, error) {
	found, err := matchingHandlers(s.hooks[event], tool)
	if err != nil {
		return hookHandler{}, err
	}
	if len(found) != 1 {
		return hookHandler{}, fmt.Errorf("expected one installed handler: %w", ErrAcceptance)
	}
	got := found[0]
	if got.Type != keyCommand || got.Command != binaryName ||
		!slices.Equal(got.Args, []string{commandHook, agentClaude}) ||
		got.Timeout != 10 || got.OnFailure != "block" {
		return hookHandler{}, fmt.Errorf("invalid installed exec form: %w", ErrAcceptance)
	}
	return got, nil
}

func matchingHandlers(groups []hookGroup, tool string) ([]hookHandler, error) {
	var found []hookHandler
	for _, group := range groups {
		matched, err := regexp.MatchString("^(?:"+group.Matcher+")$", tool)
		if err != nil {
			return nil, fmt.Errorf("match installed tool: %w", err)
		}
		if !matched {
			continue
		}
		for _, handler := range group.Hooks {
			if handler.Command == binaryName {
				found = append(found, handler)
			}
		}
	}
	return found, nil
}

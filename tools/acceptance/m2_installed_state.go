package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	m2IndexFilename = "index"
	m2IndexVariable = "GIT_INDEX_FILE"
)

// Lefthook refreshes index stat data while hiding and restoring unstaged files,
// so installed rungs compare index entries instead of administrative bytes.
func (s *suite) captureM2Installed(ctx context.Context, repo string, env []string) (m2State, error) {
	state, err := s.captureM2(ctx, repo, env)
	if err != nil {
		return m2State{}, err
	}
	for path := range state.Indexes {
		entries, err := s.m2IndexEntries(ctx, repo, env, path)
		if err != nil {
			return m2State{}, err
		}
		state.Indexes[path] = entries
	}
	return state, nil
}

func (s *suite) m2IndexEntries(ctx context.Context, repo string, env []string, path string) (string, error) {
	index, err := readFile(path)
	if err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp(s.base, "m2-index-entries-")
	if err != nil {
		return "", fmt.Errorf("create M2 index entries directory: %w", err)
	}
	entries, listErr := s.m2IndexEntriesAt(ctx, repo, env, index, dir)
	if removeErr := os.RemoveAll(dir); removeErr != nil {
		return "", fmt.Errorf("remove M2 index entries copy: %w", errors.Join(listErr, removeErr))
	}
	return entries, listErr
}

func (s *suite) m2IndexEntriesAt(ctx context.Context, repo string, env []string, index []byte, dir string) (string, error) {
	path := filepath.Join(dir, m2IndexFilename)
	if err := writeFile(path, index, fileMode); err != nil {
		return "", err
	}
	env = slices.DeleteFunc(slices.Clone(env), func(entry string) bool {
		key, _, _ := strings.Cut(entry, "=")
		return strings.EqualFold(key, m2IndexVariable) || strings.EqualFold(key, "GIT_OPTIONAL_LOCKS")
	})
	got := s.execute(ctx, command{
		program: s.git, args: []string{"ls-files", "--stage", "-z"}, dir: repo,
		env: append(env, "GIT_INDEX_FILE="+path, "GIT_OPTIONAL_LOCKS=0"),
	})
	if err := success(got); err != nil {
		return "", fmt.Errorf("list M2 copied index entries: %w", err)
	}
	return got.stdout, nil
}

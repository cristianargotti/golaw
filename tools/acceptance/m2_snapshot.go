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

	"github.com/cristianargotti/golaw/internal/install"
)

const gitDirectory = ".git"

type m2State struct {
	Head    string
	Tree    string
	Files   map[string]string
	Indexes map[string]string
}

func (s *suite) captureM2(ctx context.Context, repo string, env []string) (m2State, error) {
	gitDir, err := s.m2ObserveGit(ctx, repo, env, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return m2State{}, err
	}
	head, err := s.m2ObserveGit(ctx, repo, env, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return m2State{}, err
	}
	indexes, selected, err := m2ReadIndexes(repo, gitDir, env)
	if err != nil {
		return m2State{}, err
	}
	tree, err := s.m2SnapshotTree(ctx, repo, env, selected)
	if err != nil {
		return m2State{}, err
	}
	files, err := m2CheckoutDigests(repo, gitDir)
	if err != nil {
		return m2State{}, err
	}
	return m2State{Head: head, Tree: tree, Files: files, Indexes: indexes}, nil
}

func (s *suite) m2ObserveGit(ctx context.Context, repo string, env []string, args ...string) (string, error) {
	env = slices.DeleteFunc(slices.Clone(env), func(entry string) bool {
		key, _, _ := strings.Cut(entry, "=")
		return strings.EqualFold(key, "GIT_OPTIONAL_LOCKS")
	})
	got := s.execute(ctx, command{
		program: s.git, args: append([]string{"-c", "core.quotePath=false"}, args...),
		dir: repo, env: append(env, "GIT_OPTIONAL_LOCKS=0"),
	})
	if err := success(got); err != nil {
		return "", err
	}
	value := strings.TrimSpace(got.stdout)
	if value == "" {
		return "", fmt.Errorf("empty M2 git observation: %w", ErrAcceptance)
	}
	return value, nil
}

// Git writes cache-tree extensions even when optional locks are disabled,
// so the tree probe receives a disposable copy of the selected index.
func (s *suite) m2SnapshotTree(ctx context.Context, repo string, env []string, index []byte) (string, error) {
	dir, err := os.MkdirTemp(s.base, "m2-index-")
	if err != nil {
		return "", fmt.Errorf("create M2 index copy directory: %w", err)
	}
	tree, treeErr := s.m2SnapshotTreeAt(ctx, repo, env, index, dir)
	if removeErr := os.RemoveAll(dir); removeErr != nil {
		return "", fmt.Errorf("remove M2 index copy: %w", errors.Join(treeErr, removeErr))
	}
	return tree, treeErr
}

func (s *suite) m2SnapshotTreeAt(ctx context.Context, repo string, env []string, index []byte, dir string) (string, error) {
	path := filepath.Join(dir, "index")
	if err := writeFile(path, index, fileMode); err != nil {
		return "", err
	}
	env = slices.DeleteFunc(slices.Clone(env), func(entry string) bool {
		key, _, _ := strings.Cut(entry, "=")
		return strings.EqualFold(key, "GIT_INDEX_FILE")
	})
	return s.m2ObserveGit(ctx, repo, append(env, "GIT_INDEX_FILE="+path), "write-tree")
}

func m2ReadIndexes(repo, gitDir string, env []string) (map[string]string, []byte, error) {
	paths := []string{filepath.Join(gitDir, "index")}
	if alternate := environmentValue(env, "GIT_INDEX_FILE"); alternate != "" {
		if !filepath.IsAbs(alternate) {
			alternate = filepath.Join(repo, alternate)
		}
		paths = append(paths, filepath.Clean(alternate))
	}
	digests := make(map[string]string, len(paths))
	var selected []byte
	for _, path := range paths {
		body, err := readFile(path)
		if err != nil {
			return nil, nil, err
		}
		digests[path] = install.Digest(body)
		selected = body
	}
	return digests, selected, nil
}

func m2CheckoutDigests(repo, gitDir string) (map[string]string, error) {
	out := make(map[string]string)
	err := filepath.WalkDir(repo, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Name() == gitDirectory || path == gitDir {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		body, err := m2CheckoutBytes(path, entry.Type())
		if err != nil {
			return err
		}
		out[path] = install.Digest(body)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("capture M2 checkout: %w", err)
	}
	return out, nil
}

func m2CheckoutBytes(path string, mode fs.FileMode) ([]byte, error) {
	if mode&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			return nil, fmt.Errorf("read M2 checkout link: %w", err)
		}
		return []byte("symlink:" + target), nil
	}
	if !mode.IsRegular() {
		return nil, fmt.Errorf("unsupported M2 checkout entry: %w", ErrAcceptance)
	}
	body, err := readFile(path)
	if err != nil {
		return nil, err
	}
	return append([]byte("file:"), body...), nil
}

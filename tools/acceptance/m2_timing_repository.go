package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cristianargotti/golaw/internal/rung"
)

const (
	m2TimingSourcePath = "internal/source/doc.go"
	m2TimingLine       = "// Frozen inputs keep measurement independent from checkout edits.\n"
	m2TimingListFiles  = "ls-files"
)

func (s *suite) m2TimingSource(ctx context.Context) ([]m2ArchiveFile, error) {
	got := s.execute(ctx, command{
		program: s.git, args: []string{m2TimingListFiles, "-z"}, dir: s.root, env: s.env,
	})
	if err := success(got); err != nil {
		return nil, err
	}
	paths := strings.Split(strings.TrimSuffix(got.stdout, "\x00"), "\x00")
	return m2ArchiveFiles(s.root, paths)
}

func (s *suite) m2TimingRepository(ctx context.Context, rungName string) (string, error) {
	files, err := s.m2TimingSource(ctx)
	if err != nil {
		return "", err
	}
	repo, err := s.repository(ctx, "kit", false)
	if err != nil {
		return "", err
	}
	if copyErr := s.m2CopyTimingSource(repo, files); copyErr != nil {
		return "", copyErr
	}
	if bootstrapErr := s.m2TimingBootstrap(ctx, repo); bootstrapErr != nil {
		return "", bootstrapErr
	}
	path := filepath.Join(repo, filepath.FromSlash(m2TimingSourcePath))
	body, err := readFile(path)
	if err != nil {
		return "", err
	}
	if err := writeFile(path, append(body, []byte(m2TimingLine)...), fileMode); err != nil {
		return "", err
	}
	if err := s.gitSteps(ctx, repo, []string{gitAdd, "--", m2TimingSourcePath}); err != nil {
		return "", err
	}
	if rungName == rung.PrePush {
		if err := s.gitSteps(ctx, repo, commitArgs("test: prepare measured push")); err != nil {
			return "", err
		}
	}
	return repo, nil
}

func (s *suite) m2CopyTimingSource(repo string, files []m2ArchiveFile) error {
	for _, file := range files {
		info, err := os.Stat(filepath.Join(s.root, filepath.FromSlash(file.name)))
		if err != nil {
			return fmt.Errorf("inspect timing source mode: %w", err)
		}
		mode := os.FileMode(fileMode)
		if info.Mode().Perm()&0o111 != 0 {
			mode = execMode
		}
		if err := writeFile(filepath.Join(repo, filepath.FromSlash(file.name)), file.data, mode); err != nil {
			return err
		}
	}
	return nil
}

func (s *suite) m2TimingBootstrap(ctx context.Context, repo string) error {
	if err := s.gitSteps(ctx, repo, []string{commandInit, m2Quiet}); err != nil {
		return err
	}
	if err := m2TimingPin(repo); err != nil {
		return err
	}
	// The kit's own hooks are not the adopter asset whose installed rungs are timed.
	if err := os.Remove(filepath.Join(repo, "lefthook.yml")); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("replace timing hook configuration: %w", err)
	}
	return s.m2LifecycleBaseline(ctx, repo)
}

func m2TimingPin(repo string) error {
	path := filepath.Join(repo, policyFile)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect timing policy: %w", err)
	}
	body, err := m2VersionPolicy(repo, m2CandidateVersion)
	if err != nil {
		return err
	}
	return writeFile(path, body, fileMode)
}

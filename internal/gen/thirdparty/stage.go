package thirdparty

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/source"
)

// The modes of a stage: it lives under a temporary directory of the
// person running golaw, so nobody else reads it.
const (
	stageDirMode  = 0o700
	stageFileMode = 0o600
)

// stageError is the shape of every stage failure: the path, then the cause.
const stageError = "stage %s: %w"

// Stage copies into dir the generator's configuration and the inputs it
// reads, and prepares its output directories, so a check run writes into
// the stage and never into the checkout.
func Stage(s model.Snapshot, g Generator, dir string) error {
	for _, file := range model.Select(s, func(p string) bool { return p == g.Config || feeds(g, p) }) {
		if err := stageFile(s, dir, file.Path); err != nil {
			return err
		}
	}
	for _, out := range g.Outputs {
		target, err := stagePath(dir, out)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(target, stageDirMode); err != nil {
			return fmt.Errorf(stageError, out, err)
		}
	}
	return nil
}

// stageFile writes one snapshot file under the stage as the snapshot reads
// it, so an edit not yet committed is what the generator sees.
func stageFile(s model.Snapshot, dir, rel string) error {
	content, err := s.Read(rel)
	if err != nil {
		return fmt.Errorf(stageError, rel, err)
	}
	target, err := stagePath(dir, rel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), stageDirMode); err != nil {
		return fmt.Errorf(stageError, rel, err)
	}
	if err := os.WriteFile(target, content, stageFileMode); err != nil {
		return fmt.Errorf(stageError, rel, err)
	}
	return nil
}

// stagePath is the filesystem path of a slash path under the stage; a path
// that would leave it is refused before anything is written.
func stagePath(dir, rel string) (string, error) {
	cleaned, err := source.CleanRel(rel)
	if err != nil {
		return "", fmt.Errorf(stageError, rel, err)
	}
	return filepath.Join(dir, filepath.FromSlash(cleaned)), nil
}

// Compare lists, sorted, every output path whose bytes in the checkout
// differ from what the generator wrote into the stage, or that only one
// side holds.
func Compare(s model.Snapshot, g Generator, dir string) ([]string, error) {
	var drift []string
	for _, out := range g.Outputs {
		staged, err := stagedFiles(dir, out)
		if err != nil {
			return nil, err
		}
		checkout := model.Select(s, func(p string) bool { return strings.HasPrefix(p, out+"/") })
		for _, file := range checkout {
			want, err := s.Read(file.Path)
			if err != nil {
				return nil, fmt.Errorf("compare %s: %w", file.Path, err)
			}
			if got, ok := staged[file.Path]; !ok || !bytes.Equal(got, want) {
				drift = append(drift, file.Path)
			}
			delete(staged, file.Path)
		}
		drift = slices.AppendSeq(drift, maps.Keys(staged))
	}
	slices.Sort(drift)
	return slices.Compact(drift), nil
}

// stagedFiles reads every regular file the generator wrote under one output
// of the stage, keyed by its slash path from the stage root; an output the
// run never created holds nothing.
func stagedFiles(dir, out string) (map[string][]byte, error) {
	files := make(map[string][]byte)
	root := filepath.Join(dir, filepath.FromSlash(out))
	err := filepath.WalkDir(root, func(p string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if p == root && errors.Is(walkErr, fs.ErrNotExist) {
				return fs.SkipAll
			}
			return walkErr
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return fmt.Errorf("relate %s: %w", p, err)
		}
		content, err := os.ReadFile(p) //nolint:gosec // the path comes from walking the stage golaw created
		if err != nil {
			return fmt.Errorf("read %s: %w", p, err)
		}
		files[path.Clean(filepath.ToSlash(rel))] = content
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("compare %s: %w", out, err)
	}
	return files, nil
}

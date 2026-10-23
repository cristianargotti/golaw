package main

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// m5Files reads the fixture files of a directory of the module, each
// stored with a .txt suffix so no tool of the kit reads it as its own
// configuration, keyed by slash path without the suffix.
func m5Files(root, dir string) (map[string][]byte, error) {
	base := filepath.Join(root, filepath.FromSlash(dir))
	files := make(map[string][]byte)
	err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || !entry.Type().IsRegular() {
			return walkErr
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return fmt.Errorf("relativize the M5 fixture file: %w", err)
		}
		name, found := strings.CutSuffix(filepath.ToSlash(rel), ".txt")
		if !found {
			return fmt.Errorf("the M5 fixture file %s lacks the .txt suffix: %w", rel, ErrAcceptance)
		}
		body, err := readFile(path)
		if err != nil {
			return err
		}
		files[name] = body
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("read the M5 fixture %s: %w", dir, err)
	}
	return files, nil
}

// m5Overlay writes the files of a fixture directory into a repository.
func m5Overlay(root, dir, repo string) error {
	files, err := m5Files(root, dir)
	if err != nil {
		return err
	}
	for path, body := range files {
		if stepErr := writeFile(filepath.Join(repo, filepath.FromSlash(path)), body, fileMode); stepErr != nil {
			return stepErr
		}
	}
	return nil
}

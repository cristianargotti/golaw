package main

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

const m3BirthDir = "tools/acceptance/testdata/m3/template/birth"

func m3BirthFiles(root string) (map[string][]byte, error) {
	dir := filepath.Join(root, filepath.FromSlash(m3BirthDir))
	files := make(map[string][]byte)
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || !entry.Type().IsRegular() {
			return walkErr
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return fmt.Errorf("relativize birth fixture file: %w", err)
		}
		rel = filepath.ToSlash(rel)
		name, found := strings.CutSuffix(rel, ".txt")
		if !found {
			return fmt.Errorf("birth fixture file %s lacks the .txt suffix: %w", rel, ErrAcceptance)
		}
		body, err := readFile(path)
		if err != nil {
			return err
		}
		files[name] = body
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("read the birth fixture: %w", err)
	}
	return files, nil
}

func m3WriteBirth(root, dir string) error {
	files, err := m3BirthFiles(root)
	if err != nil {
		return err
	}
	for path, body := range files {
		if err := writeFile(filepath.Join(dir, filepath.FromSlash(path)), body, fileMode); err != nil {
			return err
		}
	}
	return nil
}

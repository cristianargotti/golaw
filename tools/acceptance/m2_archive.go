package main

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type m2ArchiveFile struct {
	name string
	data []byte
}

func writeM2Archive(out io.Writer, version string, files []m2ArchiveFile) error {
	archive := zip.NewWriter(out)
	err := m2ArchiveEntries(archive, version, files)
	closeErr := archive.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return fmt.Errorf("close M2 module archive: %w", closeErr)
	}
	return nil
}

func m2ArchiveEntries(archive *zip.Writer, version string, files []m2ArchiveFile) error {
	for _, file := range files {
		if !m2ArchivePath(file.name) {
			return fmt.Errorf("invalid M2 archive path %q: %w", file.name, ErrAcceptance)
		}
		entry, err := archive.Create(m2ModulePath + "@v" + version + "/" + file.name)
		if err != nil {
			return fmt.Errorf("create M2 archive entry %q: %w", file.name, err)
		}
		if _, err := entry.Write(file.data); err != nil {
			return fmt.Errorf("write M2 archive entry %q: %w", file.name, err)
		}
	}
	return nil
}

func m2ArchiveFiles(root string, paths []string) ([]m2ArchiveFile, error) {
	ordered := slices.Clone(paths)
	slices.Sort(ordered)
	files := make([]m2ArchiveFile, 0, len(ordered))
	seen := make(map[string]bool, len(ordered))
	for _, name := range ordered {
		key := strings.ToLower(name)
		if seen[key] || !m2ArchivePath(name) {
			return nil, fmt.Errorf("invalid or repeated M2 source path %q: %w", name, ErrAcceptance)
		}
		seen[key] = true
		body, err := m2ArchiveSource(root, name)
		if err != nil {
			return nil, err
		}
		files = append(files, m2ArchiveFile{name: name, data: body})
	}
	if !slices.Contains(paths, moduleFile) {
		return nil, fmt.Errorf("M2 source lacks tracked go.mod: %w", ErrAcceptance)
	}
	return files, nil
}

func m2ArchivePath(name string) bool {
	if name == "." || !fs.ValidPath(name) || strings.ContainsAny(name, `\:`) {
		return false
	}
	for part := range strings.SplitSeq(name, "/") {
		if strings.EqualFold(part, ".git") {
			return false
		}
	}
	return true
}

func m2ArchiveSource(root, name string) ([]byte, error) {
	path := root
	parts := strings.Split(name, "/")
	for i, part := range parts {
		path = filepath.Join(path, part)
		info, err := os.Lstat(path)
		if err != nil {
			return nil, fmt.Errorf("inspect M2 archive source %q: %w", name, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || (i == len(parts)-1 && !info.Mode().IsRegular()) {
			return nil, fmt.Errorf("M2 archive source %q is not a regular tracked file: %w", name, ErrAcceptance)
		}
	}
	return readFile(path)
}

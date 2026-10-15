package assets

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"slices"
	"strings"
)

type manifest struct {
	Files []string `json:"files"`
}

// Load permits incomplete bundles to be tested without replacing the root FS.
func Load(source fs.FS) ([]File, error) {
	names, err := manifestNames(source)
	if err != nil {
		return nil, err
	}
	files := make([]File, 0, len(names))
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		if seen[name] || !assetPath(name) {
			return nil, fmt.Errorf("%w: invalid or duplicate path %q", ErrManifest, name)
		}
		data, readErr := fs.ReadFile(source, name)
		if readErr != nil {
			return nil, fmt.Errorf("%w: read %s: %w", ErrManifest, name, readErr)
		}
		seen[name] = true
		files = append(files, File{Path: name, Data: data})
	}
	if err := complete(source, seen); err != nil {
		return nil, err
	}
	return files, nil
}

func manifestNames(source fs.FS) ([]string, error) {
	data, err := fs.ReadFile(source, "agents/manifest.json")
	if err != nil {
		return nil, fmt.Errorf("%w: read: %w", ErrManifest, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var value manifest
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("%w: decode: %w", ErrManifest, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: trailing document", ErrManifest)
	}
	if len(value.Files) == 0 || !slices.IsSorted(value.Files) {
		return nil, fmt.Errorf("%w: paths must be nonempty and sorted", ErrManifest)
	}
	return value.Files, nil
}

func assetPath(name string) bool {
	return fs.ValidPath(name) &&
		(strings.HasPrefix(name, "agents/") || strings.HasPrefix(name, "templates/"))
}

func complete(source fs.FS, seen map[string]bool) error {
	for _, root := range []string{"agents", "templates"} {
		err := fs.WalkDir(source, root, func(name string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return fmt.Errorf("%w: walk: %w", ErrManifest, walkErr)
			}
			if !entry.IsDir() && !seen[name] {
				return fmt.Errorf("%w: unlisted asset %s", ErrManifest, name)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("walk asset roots: %w", err)
		}
	}
	return nil
}

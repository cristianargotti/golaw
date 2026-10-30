// Package surfacefix serves the synthetic fixture repository of the issue
// classes a Go repository's non-Go surface met in a remote analyzer: a red
// twin that reproduces each class and a corrected twin that holds the fix
// of each. It imports no testing package, so the acceptance harness links
// it as the tests do.
package surfacefix

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/cristianargotti/golaw/internal/proof"
)

// Twin names one of the two trees.
type Twin int

// The two twins.
const (
	Red       Twin = iota + 1 // reproduces every issue class of M6.10 row (1)
	Corrected                 // the same tree after the fixes, zero findings expected
)

// tree holds both twins; all: keeps the dotfiles a repository carries.
//
//go:embed all:testdata
var tree embed.FS

// The modes Write creates the tree with.
const (
	dirMode  = 0o750
	fileMode = 0o600
)

// root names the directory of a twin inside the embedded tree.
func root(t Twin) (string, error) {
	switch t {
	case Red:
		return "testdata/red", nil
	case Corrected:
		return "testdata/corrected", nil
	default:
		return "", fmt.Errorf("%w: %d", ErrTwin, t)
	}
}

// Files answers every file of a twin by its slash path in the repository:
// a name ending in proof.EscapedSuffix loses the suffix and its escapes,
// so bytes the kit's own checks or a secret scanner would refuse never sit
// in the kit's tree as they are.
func Files(t Twin) (map[string][]byte, error) {
	dir, err := root(t)
	if err != nil {
		return nil, err
	}
	out := map[string][]byte{}
	err = fs.WalkDir(tree, dir, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		data, readErr := tree.ReadFile(name)
		if readErr != nil {
			return fmt.Errorf("read %s: %w", name, readErr)
		}
		rel := strings.TrimPrefix(name, dir+"/")
		if plain, escaped := strings.CutSuffix(rel, proof.EscapedSuffix); escaped {
			rel, data = plain, proof.Unescape(data)
		}
		out[rel] = data
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("fixture twin %d: %w", t, err)
	}
	return out, nil
}

// Write materializes a twin under dir, which must be absent or hold
// nothing but the .git directory of a repository a test just created.
func Write(dir string, t Twin) error {
	entries, err := os.ReadDir(dir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("read %s: %w", dir, err)
	}
	for _, entry := range entries {
		if entry.Name() != ".git" {
			return fmt.Errorf("%w: %s holds %s", ErrNotEmpty, dir, entry.Name())
		}
	}
	files, err := Files(t)
	if err != nil {
		return err
	}
	for rel, data := range files {
		target := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), dirMode); err != nil {
			return fmt.Errorf("create %s: %w", path.Dir(rel), err)
		}
		if err := os.WriteFile(target, data, fileMode); err != nil {
			return fmt.Errorf("write %s: %w", rel, err)
		}
	}
	return nil
}

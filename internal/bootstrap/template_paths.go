package bootstrap

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/cristianargotti/golaw/internal/install"
)

func templateRoot(root string) (string, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("bootstrap: resolve template root: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("bootstrap: resolve template root: %w", err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("bootstrap: inspect template root: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("bootstrap: template root is not a directory: %w", install.ErrPath)
	}
	return resolved, nil
}

func templatePath(root, name string) (string, error) {
	if name == "." || !fs.ValidPath(name) || strings.ContainsAny(name, "\\:\x00") {
		return "", fmt.Errorf("bootstrap: template destination %q: %w", name, install.ErrPath)
	}
	if err := templateComponent(root, ".", true); err != nil {
		return "", err
	}
	path := root
	parts := strings.Split(name, "/")
	for i, part := range parts {
		path = filepath.Join(path, part)
		if err := templateComponent(path, name, i < len(parts)-1); err != nil {
			return "", err
		}
	}
	return path, nil
}

func templateComponent(path, name string, parent bool) error {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("bootstrap: inspect template %q: %w", name, err)
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return fmt.Errorf("bootstrap: template %q: %w", name, ErrSymlink)
	}
	if (parent && !info.IsDir()) || (!parent && !info.Mode().IsRegular()) {
		return fmt.Errorf("bootstrap: template component %q: %w", name, install.ErrPath)
	}
	return nil
}

func readTemplate(root, name string) ([]byte, bool, error) {
	path, err := templatePath(root, name)
	if err != nil {
		return nil, false, err
	}
	data, err := os.ReadFile(path) //nolint:gosec // Every relative component was checked below the canonical root.
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("bootstrap: read template %q: %w", name, err)
	}
	return data, true, nil
}

func verifyTemplate(root string, change install.Change) error {
	current, existed, err := readTemplate(root, change.Path)
	if err != nil {
		return err
	}
	if change.Remove {
		return fmt.Errorf("bootstrap: template removal %q: %w", change.Path, install.ErrPath)
	}
	if existed != change.Existed || !bytes.Equal(current, change.Before) {
		return fmt.Errorf("bootstrap: template %q: %w", change.Path, install.ErrChanged)
	}
	return nil
}

func verifyTemplates(root string, plan TemplatePlan) error {
	if plan.root != "" && plan.root != root {
		return fmt.Errorf("bootstrap: template root: %w", install.ErrChanged)
	}
	for _, changes := range [][]install.Change{plan.inputs, plan.Changes, plan.Proposed} {
		for _, change := range changes {
			if err := verifyTemplate(root, change); err != nil {
				return err
			}
		}
	}
	return nil
}

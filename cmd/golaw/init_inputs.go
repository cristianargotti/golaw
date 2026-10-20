package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/install"
)

// prepareInitSnapshot binds preparation to the policy observed before compile.
func (a *app) prepareInitSnapshot(path string, opts initOptions) (string, []byte, []byte, error) {
	root, err := bootstrap.Root(path)
	if err != nil {
		return "", nil, nil, initFailed(err)
	}
	current, err := bootstrap.Current(root)
	if err != nil {
		return "", nil, nil, initFailed(err)
	}
	preparedRoot, data, _, err := a.prepareInit(root, opts)
	if err != nil {
		return "", nil, nil, err
	}
	if preparedRoot != root {
		return "", nil, nil, initFailed(fmt.Errorf("%w: repository root", install.ErrChanged))
	}
	if err := unchangedInitPolicy(root, current); err != nil {
		return "", nil, nil, initFailed(err)
	}
	return root, current, data, nil
}

const initModuleFile = "go.mod"

func readInitModule(root string) (install.Change, error) {
	data, err := os.ReadFile(filepath.Join(root, initModuleFile)) //nolint:gosec // This is the fixed module input under the resolved root.
	if errors.Is(err, fs.ErrNotExist) {
		return install.Change{Path: initModuleFile}, nil
	}
	if err != nil {
		return install.Change{}, initOperation("read module input", err)
	}
	return install.Change{Path: initModuleFile, Before: data, Existed: true}, nil
}

func unchangedInitModule(root string, before install.Change) error {
	current, err := readInitModule(root)
	if err != nil {
		return err
	}
	if current.Existed != before.Existed || !bytes.Equal(current.Before, before.Before) {
		return fmt.Errorf("%w: %s", install.ErrChanged, initModuleFile)
	}
	return nil
}

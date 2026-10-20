package runner

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Tree keeps isolated payload bytes separate from ownership metadata.
type Tree struct {
	Dir string
}

// Close validates ownership before removing an isolated tree.
func (t *Tree) Close() error {
	if t == nil || !filepath.IsAbs(t.Dir) || filepath.Clean(t.Dir) != t.Dir || filepath.Base(t.Dir) != treePayload {
		return fmt.Errorf("invalid owned tree: %w", os.ErrInvalid)
	}
	envelope := filepath.Dir(t.Dir)
	name := filepath.Base(envelope)
	if treeCreator(name) != os.Getpid() {
		return fmt.Errorf("tree creator does not own %s: %w", t.Dir, os.ErrInvalid)
	}
	parent, err := openTreeParent(filepath.Dir(envelope))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return errors.Join(removeTreeEnvelope(parent, name, false), parent.Close())
}

// Sweep keeps trees unless the creator is certainly gone.
func Sweep(parent string) error {
	path, err := resolveTreeParent(parent)
	if err != nil {
		return err
	}
	root, err := openTreeParent(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return errors.Join(sweepTreeEnvelopes(root), root.Close())
}

func sweepTreeEnvelopes(root *os.Root) error {
	names, err := treeEntries(root)
	if err != nil {
		return err
	}
	var failures []error
	for _, name := range names {
		pid := treeCreator(name)
		if pid == 0 || !treeProcessGone(pid) {
			continue
		}
		if err := removeTreeEnvelope(root, name, true); err != nil &&
			!errors.Is(err, os.ErrNotExist) && !errors.Is(err, os.ErrInvalid) {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func removeTreeEnvelope(parent *os.Root, name string, partial bool) error {
	root, info, err := openTreeEnvelope(parent, name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if ownerErr := verifyTreeOwner(root, name, partial); ownerErr != nil {
		return errors.Join(ownerErr, root.Close())
	}
	if clearErr := clearTreeEnvelope(root); clearErr != nil {
		return errors.Join(clearErr, root.Close())
	}
	if closeErr := root.Close(); closeErr != nil {
		return fmt.Errorf("close tree envelope %s: %w", name, closeErr)
	}
	current, err := parent.Lstat(name)
	if err != nil {
		return fmt.Errorf("inspect cleared tree %s: %w", name, err)
	}
	if !os.SameFile(info, current) {
		return fmt.Errorf("tree envelope changed %s: %w", name, os.ErrInvalid)
	}
	if err := parent.Remove(name); err != nil {
		return fmt.Errorf("remove tree envelope %s: %w", name, err)
	}
	return nil
}

func openTreeEnvelope(parent *os.Root, name string) (*os.Root, os.FileInfo, error) {
	info, err := parent.Lstat(name)
	if err != nil {
		return nil, nil, fmt.Errorf("inspect tree envelope %s: %w", name, err)
	}
	if !info.IsDir() {
		return nil, nil, fmt.Errorf("tree envelope is not a directory %s: %w", name, os.ErrInvalid)
	}
	root, err := parent.OpenRoot(name)
	if err != nil {
		return nil, nil, fmt.Errorf("open tree envelope %s: %w", name, err)
	}
	opened, err := root.Stat(".")
	if err == nil && !os.SameFile(info, opened) {
		err = os.ErrInvalid
	}
	if err != nil {
		return nil, nil, errors.Join(fmt.Errorf("tree envelope changed %s: %w", name, err), root.Close())
	}
	return root, info, nil
}

func treeEntries(root *os.Root) ([]string, error) {
	file, err := root.Open(".")
	if err != nil {
		return nil, fmt.Errorf("open tree directory %s: %w", root.Name(), err)
	}
	names, readErr := file.Readdirnames(-1)
	if err := errors.Join(readErr, file.Close()); err != nil {
		return nil, fmt.Errorf("read tree directory %s: %w", root.Name(), err)
	}
	return names, nil
}

func clearTreeEnvelope(root *os.Root) error {
	names, err := treeEntries(root)
	if err != nil {
		return err
	}
	for _, name := range names {
		if name == treeOwner {
			continue
		}
		if err := root.RemoveAll(name); err != nil {
			return fmt.Errorf("remove tree content %s: %w", filepath.Join(root.Name(), name), err)
		}
	}
	if err := root.Remove(treeOwner); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove tree ownership %s: %w", root.Name(), err)
	}
	return nil
}

func resolveTreeParent(parent string) (string, error) {
	if parent == "" {
		parent = os.TempDir()
	}
	absolute, err := filepath.Abs(parent)
	if err != nil {
		return "", fmt.Errorf("resolve tree parent %s: %w", parent, err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err == nil {
		return resolved, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("resolve tree links %s: %w", parent, err)
	}
	ancestor := filepath.Dir(absolute)
	if ancestor == absolute {
		return "", fmt.Errorf("resolve tree root %s: %w", parent, err)
	}
	resolved, err = resolveTreeParent(ancestor)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolved, filepath.Base(absolute)), nil
}

package runner

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"

	"github.com/cristianargotti/golaw/internal/model"
)

// Supported hosts cannot store a 64 KiB link target; cap the temporary string.
const maxTreeLinkBytes = 64 << 10

const errWindowsLinkPrivilege syscall.Errno = 1314

// maxLinkExpansions bounds the links the validation of one link follows. No
// host follows more for one path (Linux 40, the BSDs and macOS 32, Windows
// 63), so a target that needs more never resolves there and is kept as the
// loop the host sees, while links that name each other twice per target
// can no longer make the walk grow without end.
const maxLinkExpansions = 64

// linkWalk is the state of the validation of one link: the links being
// expanded, which recognize a true cycle, and the expansions left.
type linkWalk struct {
	active map[string]bool
	left   int
}

func createTreeLinks(root *os.Root, entries []treeEntry) error {
	for i := range entries {
		if entries[i].mode == "120000" {
			if err := replaceTreeLink(root, entries[i].path); err != nil {
				return err
			}
		}
	}
	for i := range entries {
		if entries[i].mode != "120000" {
			continue
		}
		name := filepath.FromSlash(entries[i].path)
		base := filepath.Dir(name)
		var parts []string
		if base != "." {
			parts = strings.Split(base, string(os.PathSeparator))
		}
		walk := &linkWalk{active: map[string]bool{}, left: maxLinkExpansions}
		if _, _, err := expandTreeLink(root, parts, name, walk); err != nil {
			return fmt.Errorf("validate symlink %q: %w", entries[i].path, err)
		}
	}
	return nil
}

func exactTreeLink(root *os.Root, name, want string) error {
	got, err := root.Readlink(name)
	if err != nil {
		return fmt.Errorf("read created symlink %q: %w", name, err)
	}
	if got != want {
		return fmt.Errorf("%w: host transforms the target text of symlink %q", model.ErrCannotJudge, name)
	}
	return nil
}

func replaceTreeLink(root *os.Root, name string) error {
	file, err := root.Open(name)
	if err != nil {
		return fmt.Errorf("open symlink payload %q: %w", name, err)
	}
	data, readErr := io.ReadAll(io.LimitReader(file, maxTreeLinkBytes+1))
	if err := errors.Join(readErr, file.Close()); err != nil {
		return fmt.Errorf("read symlink payload %q: %w", name, err)
	}
	if len(data) > maxTreeLinkBytes {
		return fmt.Errorf("%w: symlink %q target exceeds host capacity of %d bytes", model.ErrCannotJudge, name, maxTreeLinkBytes)
	}
	if err := root.Remove(name); err != nil {
		return fmt.Errorf("remove symlink payload %q: %w", name, err)
	}
	if err := root.Symlink(string(data), name); err != nil {
		if len(data) == 0 || linkHostLimitation(err) {
			return fmt.Errorf("%w: host cannot create symlink %q: %w", model.ErrCannotJudge, name, err)
		}
		return fmt.Errorf("create symlink %q: %w", name, err)
	}
	return exactTreeLink(root, name, string(data))
}

func linkHostLimitation(err error) bool {
	for _, limitation := range []error{os.ErrPermission, errors.ErrUnsupported, syscall.ENAMETOOLONG, syscall.EINVAL} {
		if errors.Is(err, limitation) {
			return true
		}
	}
	return runtime.GOOS == "windows" && errors.Is(err, errWindowsLinkPrivilege)
}

func expandTreeLink(root *os.Root, base []string, name string, walk *linkWalk) ([]string, bool, error) {
	if walk.active[name] || walk.left == 0 {
		return nil, true, nil
	}
	walk.left--
	walk.active[name] = true
	defer delete(walk.active, name)
	target, err := root.Readlink(name)
	if err != nil {
		return nil, false, fmt.Errorf("read symlink %q: %w", name, err)
	}
	if target == "" {
		return nil, true, nil
	}
	target = filepath.FromSlash(target)
	if filepath.IsAbs(target) || filepath.VolumeName(target) != "" || strings.HasPrefix(target, string(os.PathSeparator)) {
		return nil, false, fmt.Errorf("%w: symlink %q has an absolute target", model.ErrCannotJudge, name)
	}
	return resolveTreeLink(root, base, strings.Split(target, string(os.PathSeparator)), walk)
}

func resolveTreeLink(root *os.Root, base, parts []string, walk *linkWalk) ([]string, bool, error) {
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		var stopped bool
		var err error
		if part == ".." {
			base, stopped, err = parentTreeLink(root, base)
		} else {
			base, stopped, err = stepTreeLink(root, base, part, walk)
		}
		if stopped || err != nil {
			return nil, stopped, err
		}
	}
	return base, false, nil
}

func stepTreeLink(root *os.Root, base []string, part string, walk *linkWalk) ([]string, bool, error) {
	candidate := slices.Clone(base)
	candidate = append(candidate, part)
	name := filepath.Join(candidate...)
	info, err := root.Lstat(name)
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
		return nil, true, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("inspect symlink target %q: %w", name, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return candidate, false, nil
	}
	return expandTreeLink(root, base, name, walk)
}

func parentTreeLink(root *os.Root, base []string) ([]string, bool, error) {
	if len(base) == 0 {
		return nil, false, fmt.Errorf("%w: symlink target traverses above the tree", model.ErrCannotJudge)
	}
	name := filepath.Join(base...)
	info, err := root.Lstat(name)
	if err != nil {
		return nil, false, fmt.Errorf("inspect symlink parent %q: %w", name, err)
	}
	if !info.IsDir() {
		return nil, true, nil
	}
	return base[:len(base)-1], false, nil
}

package runner

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

const (
	treePrefix        = "golaw-tree-"
	treePayload       = "tree"
	treeOwner         = "owner"
	treeOwnerPending  = "owner.tmp"
	treeHead          = "HEAD"
	treeNonceBytes    = 16
	maxTreeOwnerBytes = 128
)

func newTree(parent string) (*Tree, error) {
	parent, err := resolveTreeParent(parent)
	if err != nil {
		return nil, err
	}
	if repoErr := rejectTreeRepository(parent); repoErr != nil {
		return nil, repoErr
	}
	if mkdirErr := os.MkdirAll(parent, lockDirMode); mkdirErr != nil {
		return nil, fmt.Errorf("create tree parent %s: %w", parent, mkdirErr)
	}
	root, err := openTreeParent(parent)
	if err != nil {
		return nil, err
	}
	tree, createErr := createTree(root, parent)
	closeErr := root.Close()
	if closeErr != nil && tree != nil {
		return nil, errors.Join(createErr, closeErr, tree.Close())
	}
	return tree, errors.Join(createErr, closeErr)
}

func createTree(root *os.Root, parent string) (*Tree, error) {
	var nonce [treeNonceBytes]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, fmt.Errorf("create tree nonce: %w", err)
	}
	name := treePrefix + strconv.Itoa(os.Getpid()) + "-" + hex.EncodeToString(nonce[:])
	if err := root.Mkdir(name, lockDirMode); err != nil {
		return nil, fmt.Errorf("create tree envelope %s: %w", name, err)
	}
	if err := publishTree(root, name); err != nil {
		return nil, errors.Join(err, root.RemoveAll(name))
	}
	return &Tree{Dir: filepath.Join(parent, name, treePayload)}, nil
}

func publishTree(parent *os.Root, name string) error {
	root, err := parent.OpenRoot(name)
	if err != nil {
		return fmt.Errorf("open new tree %s: %w", name, err)
	}
	return errors.Join(writeTreeOwner(root, name), root.Close())
}

func writeTreeOwner(root *os.Root, name string) error {
	file, err := root.OpenFile(treeOwnerPending, os.O_CREATE|os.O_EXCL|os.O_WRONLY, lockMode)
	if err != nil {
		return fmt.Errorf("create tree owner %s: %w", name, err)
	}
	_, writeErr := file.WriteString(treeOwnership(name))
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return fmt.Errorf("write tree owner %s: %w", name, err)
	}
	if err := root.Rename(treeOwnerPending, treeOwner); err != nil {
		return fmt.Errorf("publish tree owner %s: %w", name, err)
	}
	if err := root.Mkdir(treePayload, lockDirMode); err != nil {
		return fmt.Errorf("create tree payload %s: %w", name, err)
	}
	return nil
}

func treeOwnership(name string) string { return "golaw-tree-owner-v1\n" + name + "\n" }

func treeCreator(name string) int {
	value, found := strings.CutPrefix(name, treePrefix)
	if !found {
		return 0
	}
	pidText, nonce, found := strings.Cut(value, "-")
	if !found || len(nonce) != hex.EncodedLen(treeNonceBytes) {
		return 0
	}
	if _, err := hex.DecodeString(nonce); err != nil {
		return 0
	}
	pid, err := strconv.Atoi(pidText)
	if err != nil || pid <= 0 || strconv.Itoa(pid) != pidText {
		return 0
	}
	return pid
}

func verifyTreeOwner(root *os.Root, name string, partial bool) error {
	info, err := root.Lstat(treeOwner)
	if partial && errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect tree owner %s: %w", name, err)
	}
	if !info.Mode().IsRegular() || info.Size() > maxTreeOwnerBytes {
		return fmt.Errorf("invalid tree owner %s: %w", name, os.ErrInvalid)
	}
	file, err := root.OpenFile(treeOwner, lockOpenFlags() & ^(os.O_CREATE|os.O_RDWR), 0)
	if err != nil {
		return fmt.Errorf("open tree owner %s: %w", name, err)
	}
	opened, statErr := file.Stat()
	data, readErr := io.ReadAll(io.LimitReader(file, maxTreeOwnerBytes))
	if readErr = errors.Join(statErr, readErr, file.Close()); readErr != nil {
		return fmt.Errorf("read tree owner %s: %w", name, readErr)
	}
	if !os.SameFile(info, opened) || string(data) != treeOwnership(name) {
		return fmt.Errorf("invalid tree ownership %s: %w", name, os.ErrInvalid)
	}
	return nil
}

// rejectTreeRepository refuses a parent inside any repository, worktree or
// bare, because a git that a tool runs in the copy would answer for that
// repository. The refusal is a place the kit cannot judge in, so a rung
// leaves its Go steps to the pipeline rather than failing the commit.
func rejectTreeRepository(parent string) error {
	for current := parent; ; current = filepath.Dir(current) {
		_, err := os.Lstat(filepath.Join(current, ".git"))
		if err == nil || bareTreeRepository(current) {
			return fmt.Errorf("%w: tree parent %s is inside repository %s: %w",
				model.ErrCannotJudge, parent, current, os.ErrInvalid)
		}
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect tree parent %s: %w", current, err)
		}
		if filepath.Dir(current) == current {
			return nil
		}
	}
}

func bareTreeRepository(path string) bool {
	head, err := os.Stat(filepath.Join(path, treeHead))
	if err != nil || !head.Mode().IsRegular() {
		return false
	}
	for _, name := range []string{"objects", "refs"} {
		info, statErr := os.Stat(filepath.Join(path, name))
		if statErr != nil || !info.IsDir() {
			return false
		}
	}
	file, err := os.OpenInRoot(path, treeHead)
	if err != nil {
		return false
	}
	data, readErr := io.ReadAll(io.LimitReader(file, maxTreeOwnerBytes))
	if closeErr := file.Close(); readErr != nil || closeErr != nil {
		return false
	}
	value := strings.TrimSpace(string(data))
	ref, symbolic := strings.CutPrefix(value, "ref:")
	return validObject(value) || symbolic && strings.HasPrefix(strings.TrimSpace(ref), "refs/")
}

func openTreeParent(path string) (*os.Root, error) {
	base := filepath.VolumeName(path) + string(filepath.Separator)
	root, err := os.OpenRoot(base)
	if err != nil {
		return nil, fmt.Errorf("open tree parent %s: %w", path, err)
	}
	for part := range strings.SplitSeq(strings.TrimPrefix(path, base), string(filepath.Separator)) {
		if part == "" {
			continue
		}
		next, _, openErr := openTreeEnvelope(root, part)
		closeErr := root.Close()
		if openErr != nil {
			return nil, errors.Join(openErr, closeErr)
		}
		if closeErr != nil {
			return nil, errors.Join(closeErr, next.Close())
		}
		root = next
	}
	return root, nil
}

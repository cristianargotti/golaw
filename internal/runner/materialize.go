package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// Bounds limits the resources a rung can materialize before executing a tool.
type Bounds struct {
	Bytes int64
	Files int
}

const (
	defaultTreeBytes = 512 << 20
	defaultTreeFiles = 50000
	treeQuotePath    = "core.quotePath=false"
	treeGitProgram   = "git"
)

// DefaultBounds keeps materialization independent of the scanner's read limit.
func DefaultBounds() Bounds { return Bounds{Bytes: defaultTreeBytes, Files: defaultTreeFiles} }

// MaterializeInput preserves the caller's index and object-store environment.
type MaterializeInput struct {
	Dir    string
	Env    []string
	Rev    string
	Parent string
	Bounds Bounds
}

type blobCopy func(context.Context, io.Writer, io.Reader, int64) error

type treeEntry struct {
	path, mode, object string
	size               int64
}

// Materialize isolates object-store bytes from worktree edits and filters.
func Materialize(ctx context.Context, in MaterializeInput) (*Tree, error) {
	return materialize(ctx, in, copyTreeBlob)
}

func materialize(ctx context.Context, in MaterializeInput, copyBlob blobCopy) (*Tree, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("materialize tree: %w", err)
	}
	if in.Bounds.Bytes <= 0 || in.Bounds.Files <= 0 {
		return nil, fmt.Errorf("%w: materialization bounds must be positive", model.ErrCannotJudge)
	}
	entries, err := listTree(ctx, in)
	if err != nil {
		return nil, err
	}
	if sizeErr := sizeTree(ctx, in, entries); sizeErr != nil {
		return nil, sizeErr
	}
	if cancelErr := ctx.Err(); cancelErr != nil {
		return nil, fmt.Errorf("materialize tree: %w", cancelErr)
	}
	tree, err := newTree(in.Parent)
	if err != nil {
		return nil, err
	}
	if err := populateTree(ctx, in, tree, entries, copyBlob); err != nil {
		return nil, errors.Join(err, tree.Close())
	}
	return tree, nil
}

func populateTree(ctx context.Context, in MaterializeInput, tree *Tree, entries []treeEntry, copyBlob blobCopy) error {
	root, err := os.OpenRoot(tree.Dir)
	if err != nil {
		return fmt.Errorf("open materialized tree: %w", err)
	}
	err = writeTree(ctx, in, root, entries, copyBlob)
	if err == nil {
		err = createTreeLinks(root, entries)
	}
	return errors.Join(err, root.Close(), ctx.Err())
}

func treeCommand(in MaterializeInput, args ...string) Command {
	return Command{
		Name: treeGitProgram, Args: append([]string{"-c", treeQuotePath}, args...),
		Dir: in.Dir, Env: in.Env,
	}
}

// treeGitError names a git command of a materialization that failed with
// its status and the first line it printed: without git's answer the copy
// cannot be made, which leaves the tree unjudged rather than judged wrong.
func treeGitError(args []string, out Output) error {
	line, _, _ := strings.Cut(strings.TrimSpace(string(out.Stderr)), "\n")
	return fmt.Errorf("%w: git %s exited %d: %s", model.ErrCannotJudge, strings.Join(args, " "), out.Code, line)
}

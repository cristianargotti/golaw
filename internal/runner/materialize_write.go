package runner

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/cristianargotti/golaw/internal/model"
)

const (
	treeFileMode       = 0o644
	treeExecutableMode = 0o755
	copyTreeFailure    = "copy tree blob: %w"
)

func writeTree(ctx context.Context, in MaterializeInput, root *os.Root, entries []treeEntry, copyBlob blobCopy) error {
	for i := range entries {
		name := entries[i].path
		if entries[i].mode != "160000" {
			name = filepath.Dir(name)
		}
		if err := root.MkdirAll(name, lockDirMode); err != nil {
			return fmt.Errorf("create tree directory %q: %w", name, err)
		}
	}
	if !slices.ContainsFunc(entries, func(entry treeEntry) bool { return entry.mode != "160000" }) {
		return nil
	}
	cmd := treeCommand(in, "cat-file", "--batch")
	cmd.Stdin = treeObjectInput(entries)
	consume := func(reader io.Reader) error {
		return writeTreeStream(ctx, root, bufio.NewReader(reader), entries, copyBlob)
	}
	out, err := ReadOutput(ctx, cmd, consume)
	if err != nil {
		return fmt.Errorf("write materialized tree: %w", err)
	}
	if out.Code != 0 {
		return treeGitError(cmd.Args, out)
	}
	return nil
}

func writeTreeStream(ctx context.Context, root *os.Root, reader *bufio.Reader, entries []treeEntry, copyBlob blobCopy) error {
	for i := range entries {
		entry := entries[i]
		if entry.mode == "160000" {
			continue
		}
		size, err := readBlobHeader(reader, entry.object)
		if err != nil {
			return err
		}
		if size != entry.size {
			return fmt.Errorf("%w: blob size changed for %q", model.ErrCannotJudge, entry.path)
		}
		if writeErr := writeTreeBlob(ctx, root, reader, entry, copyBlob); writeErr != nil {
			return writeErr
		}
		separator, err := reader.ReadByte()
		if err != nil {
			return fmt.Errorf("%w: read blob separator for %q: %w", model.ErrCannotJudge, entry.path, err)
		}
		if separator != '\n' {
			return fmt.Errorf("%w: invalid blob separator for %q", model.ErrCannotJudge, entry.path)
		}
	}
	return finishTreeStream(reader)
}

func writeTreeBlob(ctx context.Context, root *os.Root, reader io.Reader, entry treeEntry, copyBlob blobCopy) error {
	mode := os.FileMode(treeFileMode)
	if entry.mode == "100755" {
		mode = treeExecutableMode
	}
	file, err := root.OpenFile(entry.path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return fmt.Errorf("create tree file %q: %w", entry.path, err)
	}
	err = copyBlob(ctx, file, reader, entry.size)
	if err == nil {
		err = file.Chmod(mode)
	}
	err = errors.Join(err, file.Close())
	if err != nil {
		return fmt.Errorf("write tree file %q: %w", entry.path, err)
	}
	return nil
}

func copyTreeBlob(ctx context.Context, dst io.Writer, src io.Reader, size int64) error {
	const chunkBytes = 32 << 10
	buffer := make([]byte, chunkBytes)
	for remaining := size; remaining > 0; {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf(copyTreeFailure, err)
		}
		n, readErr := io.ReadFull(src, buffer[:min(int64(len(buffer)), remaining)])
		written, writeErr := dst.Write(buffer[:n])
		if err := ctx.Err(); err != nil {
			return fmt.Errorf(copyTreeFailure, err)
		}
		if writeErr == nil && written != n {
			writeErr = io.ErrShortWrite
		}
		if err := errors.Join(readErr, writeErr); err != nil {
			return fmt.Errorf(copyTreeFailure, err)
		}
		remaining -= int64(n)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf(copyTreeFailure, err)
	}
	return nil
}

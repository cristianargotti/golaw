package runner

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

const treeHeaderLimit = 4096

func sizeTree(ctx context.Context, in MaterializeInput, entries []treeEntry) error {
	if !slices.ContainsFunc(entries, func(entry treeEntry) bool { return entry.mode != "160000" }) {
		return nil
	}
	cmd := treeCommand(in, "cat-file", "--batch-check")
	cmd.Stdin = treeObjectInput(entries)
	out, err := ReadOutput(ctx, cmd, func(stream io.Reader) error {
		return readTreeSizes(stream, entries, in.Bounds.Bytes)
	})
	if err != nil {
		return fmt.Errorf("measure tree objects: %w", err)
	}
	if out.Code != 0 {
		return treeGitError(cmd.Args, out)
	}
	return nil
}

func treeObjectInput(entries []treeEntry) io.Reader {
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.mode != "160000" {
			lines = append(lines, entry.object+"\n")
		}
	}
	return strings.NewReader(strings.Join(lines, ""))
}

func readTreeSizes(stream io.Reader, entries []treeEntry, bound int64) error {
	reader := bufio.NewReaderSize(stream, treeHeaderLimit)
	var total int64
	for index := range entries {
		if entries[index].mode == "160000" {
			continue
		}
		size, err := readBlobHeader(reader, entries[index].object)
		if err != nil {
			return err
		}
		total, err = addTreeSize(total, size, bound)
		if err != nil {
			return err
		}
		entries[index].size = size
	}
	return finishTreeStream(reader)
}

func addTreeSize(total, size, limit int64) (int64, error) {
	if total < 0 || size < 0 || limit < 0 {
		return 0, fmt.Errorf("%w: negative tree size or bound", model.ErrCannotJudge)
	}
	// Two nonnegative int64 values fit in uint64, including an exceeded bound.
	observed := uint64(total) + uint64(size)
	if observed > uint64(limit) {
		return 0, fmt.Errorf("%w: observed bytes %d exceed limit %d", ErrBound, observed, limit)
	}
	return total + size, nil
}

func readBlobHeader(reader *bufio.Reader, object string) (int64, error) {
	header, err := reader.ReadSlice('\n')
	if err != nil {
		return 0, fmt.Errorf("%w: read blob header for %s: %w", model.ErrCannotJudge, object, err)
	}
	if len(header) > treeHeaderLimit {
		return 0, fmt.Errorf("%w: oversized blob header for %s", model.ErrCannotJudge, object)
	}
	fields := strings.Split(strings.TrimSuffix(string(header), "\n"), " ")
	if len(fields) != 3 || fields[0] != object || fields[1] != "blob" {
		return 0, fmt.Errorf("%w: invalid blob identity or type for %s", model.ErrCannotJudge, object)
	}
	size, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: read blob size for %s: %w", model.ErrCannotJudge, object, err)
	}
	if size < 0 || strconv.FormatInt(size, 10) != fields[2] {
		return 0, fmt.Errorf("%w: invalid blob size for %s", model.ErrCannotJudge, object)
	}
	return size, nil
}

func finishTreeStream(reader *bufio.Reader) error {
	_, err := reader.ReadByte()
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w: finish object stream: %w", model.ErrCannotJudge, err)
	}
	return fmt.Errorf("%w: additional object stream bytes", model.ErrCannotJudge)
}

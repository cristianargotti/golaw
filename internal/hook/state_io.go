package hook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

func eachState(ctx context.Context, dir string, visit func(fs.DirEntry) error) error {
	//nolint:gosec // The command supplies the private state directory.
	file, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("%w: open state: %w", model.ErrCannotJudge, err)
	}
	if err := errors.Join(walkState(ctx, file, visit), file.Close()); err != nil {
		return fmt.Errorf("%w: traverse state: %w", model.ErrCannotJudge, err)
	}
	return nil
}

func walkState(ctx context.Context, file *os.File, visit func(fs.DirEntry) error) error {
	var problems error
	for {
		if err := ctx.Err(); err != nil {
			return errors.Join(problems,
				fmt.Errorf("%w: state traversal: %w", model.ErrDeadline, err))
		}
		entries, readErr := file.ReadDir(stateBatch)
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return errors.Join(problems,
					fmt.Errorf("%w: state traversal: %w", model.ErrDeadline, err))
			}
			problems = errors.Join(problems, visit(entry))
		}
		if errors.Is(readErr, io.EOF) {
			return problems
		}
		if readErr != nil {
			return errors.Join(problems,
				fmt.Errorf("%w: read state directory: %w", model.ErrCannotJudge, readErr))
		}
	}
}

func readState(path string) (stateRecord, error) {
	//nolint:gosec // The path comes from the private state listing.
	file, err := os.Open(path)
	if err != nil {
		return stateRecord{}, fmt.Errorf("%w: open record: %w", model.ErrCannotJudge, err)
	}
	data, readErr := io.ReadAll(io.LimitReader(file, maxStateBytes+1))
	if err := errors.Join(readErr, file.Close()); err != nil {
		return stateRecord{}, fmt.Errorf("%w: read record: %w", model.ErrCannotJudge, err)
	}
	if len(data) > maxStateBytes {
		return stateRecord{}, fmt.Errorf("%w: state record exceeds bound", model.ErrCannotJudge)
	}
	var record stateRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return stateRecord{}, fmt.Errorf("%w: decode state: %w", model.ErrCannotJudge, err)
	}
	prefix := record.Key + "." + record.Input + "."
	if record.Files == nil && record.Reason == "" ||
		record.Hash != MapHash(record.Files) ||
		record.ReasonHash != Hash([]byte(record.Reason)) ||
		!strings.HasPrefix(filepath.Base(path), prefix) {
		return stateRecord{}, fmt.Errorf("%w: state content identity mismatch", model.ErrCannotJudge)
	}
	return record, nil
}

func writeStateTemp(dir string, data []byte) (string, error) {
	file, err := os.CreateTemp(dir, tempPrefix)
	if err != nil {
		return "", fmt.Errorf("%w: create state: %w", model.ErrCannotJudge, err)
	}
	count, writeErr := file.Write(data)
	if count != len(data) {
		writeErr = errors.Join(writeErr, io.ErrShortWrite)
	}
	err = errors.Join(writeErr, file.Sync(), file.Close())
	if err != nil {
		return "", fmt.Errorf("%w: write state: %w", model.ErrCannotJudge,
			errors.Join(err, removeState(file.Name())))
	}
	return file.Name(), nil
}

func publishState(ctx context.Context, temp, final string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%w: publish state: %w", model.ErrDeadline,
			errors.Join(err, removeState(temp)))
	}
	if err := os.Link(temp, final); err != nil {
		return fmt.Errorf("%w: atomic state publication: %w", model.ErrCannotJudge,
			errors.Join(err, removeState(temp)))
	}
	if err := removeState(temp); err != nil {
		return fmt.Errorf("%w: remove temporary state: %w", model.ErrCannotJudge,
			errors.Join(err, removeState(final)))
	}
	return nil
}

func removeState(path string) error {
	err := os.Remove(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("remove state resource: %w", err)
	}
	return nil
}

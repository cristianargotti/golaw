package hook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cristianargotti/golaw/internal/model"
)

const (
	stateBatch    = 1024
	maxStateBytes = 32 << 20
	stateLife     = 24 * time.Hour
	tempLife      = time.Hour
	tempPrefix    = ".tmp-"
	stateDirMode  = 0o700
)

// Store receives time from command integration instead of reading a clock.
type Store struct {
	Dir string
	Now time.Time
}

// Baseline is what a pre event leaves its post event: the content
// identities of the paths git status named, the HEAD commit they were
// taken at, empty on an unborn branch, or the reason they could not be
// taken.
type Baseline struct {
	Files  map[string]string
	Head   string
	Reason string
}

type stateRecord struct {
	Key        string            `json:"key"`
	Input      string            `json:"input"`
	Hash       string            `json:"hash"`
	Files      map[string]string `json:"files"`
	Head       string            `json:"head,omitempty"`
	Reason     string            `json:"reason"`
	ReasonHash string            `json:"reason_hash"`
	At         time.Time         `json:"at"`
}

// Sweep removes expired resources by their modification time, so it never
// parses a record and never refuses a directory for its size.
func (s Store) Sweep(ctx context.Context) error {
	if err := os.MkdirAll(s.Dir, stateDirMode); err != nil {
		return fmt.Errorf("%w: state directory: %w", model.ErrCannotJudge, err)
	}
	return eachState(ctx, s.Dir, s.sweepEntry)
}

// Save publishes complete immutable bytes before a reader can discover them.
func (s Store) Save(ctx context.Context, key, inputHash string, baseline Baseline) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%w: state save: %w", model.ErrDeadline, err)
	}
	if baseline.Files == nil && baseline.Reason == "" {
		baseline.Files = map[string]string{}
	}
	data, err := stateBytes(stateRecord{
		Key: key, Input: inputHash, Files: baseline.Files, Head: baseline.Head, Reason: baseline.Reason, At: s.Now,
	})
	if err != nil {
		return err
	}
	temp, err := writeStateTemp(s.Dir, data)
	if err != nil {
		return err
	}
	name := key + "." + inputHash + "." + strings.TrimPrefix(filepath.Base(temp), tempPrefix) + ".json"
	final := filepath.Join(s.Dir, name)
	if err := publishState(ctx, temp, final); err != nil {
		return err
	}
	if _, err := s.Load(ctx, key); err != nil {
		return fmt.Errorf("%w: duplicate state: %w", model.ErrCannotJudge,
			errors.Join(err, removeState(final)))
	}
	return nil
}

// Load refuses ambiguous baselines instead of choosing a stale verdict.
func (s Store) Load(ctx context.Context, key string) (Baseline, error) {
	var found bool
	var selected stateRecord
	err := eachState(ctx, s.Dir, func(entry fs.DirEntry) error {
		if !strings.HasPrefix(entry.Name(), key+".") {
			return nil
		}
		record, err := readState(filepath.Join(s.Dir, entry.Name()))
		if err != nil {
			return err
		}
		if s.Now.Sub(record.At) >= stateLife {
			return nil
		}
		if found || record.Key != key {
			return fmt.Errorf("%w: ambiguous state identity", model.ErrCannotJudge)
		}
		found, selected = true, record
		return nil
	})
	if err != nil {
		return Baseline{}, err
	}
	if !found {
		return Baseline{}, fmt.Errorf("%w: baseline absent: %w", model.ErrCannotJudge, fs.ErrNotExist)
	}
	return Baseline{Files: selected.Files, Head: selected.Head, Reason: selected.Reason}, nil
}

func (s Store) sweepEntry(entry fs.DirEntry) error {
	path := filepath.Join(s.Dir, entry.Name())
	info, err := entry.Info()
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w: inspect state: %w", model.ErrCannotJudge, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: nonregular state resource", model.ErrCannotJudge)
	}
	life := stateLife
	if strings.HasPrefix(entry.Name(), tempPrefix) {
		life = tempLife
	}
	if s.Now.Sub(info.ModTime()) >= life {
		return removeState(path)
	}
	return nil
}

func stateBytes(record stateRecord) ([]byte, error) {
	record.Hash = MapHash(record.Files)
	record.ReasonHash = Hash([]byte(record.Reason))
	data, err := json.Marshal(record)
	if err != nil {
		return nil, fmt.Errorf("%w: encode state: %w", model.ErrCannotJudge, err)
	}
	if len(data) > maxStateBytes {
		record.Files = nil
		record.Reason = "content identity record exceeds 32 MiB"
		return stateBytes(record)
	}
	return data, nil
}

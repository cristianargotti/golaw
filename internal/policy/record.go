package policy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/cristianargotti/golaw/internal/model"
)

// recordsDir is the directory of the state directory that holds one
// enrollment record per repository, readable by its owner only.
const (
	recordsDir     = "enrollments"
	recordsDirMode = 0o700
)

// The writers an enrollment record names.
const (
	WrittenByInit    = "init"
	WrittenByInstall = "agent-install"
)

// Record is the enrollment record of A3, kept outside the checkout in the
// state directory and keyed by the repository identity: what golaw.yml
// and the profile must hash to, and the bytes of the policy it enrolled.
// Those bytes are the bootstrap policy that governs the repository while
// its branch is unborn, and the policy a re-enrollment must tighten.
type Record struct {
	Schema        int       `json:"golaw"`
	Root          string    `json:"root"`
	PolicyDigest  string    `json:"policy_digest"`
	ProfileDigest string    `json:"profile_digest,omitempty"`
	KitVersion    string    `json:"kit_version"`
	Bootstrap     string    `json:"bootstrap_policy,omitempty"`
	WrittenBy     string    `json:"written_by"`
	WrittenAt     time.Time `json:"written_at"`
}

// Identity is the repository identity a record is keyed by: the real root
// path in slash form.
func Identity(root string) string { return filepath.ToSlash(root) }

// RecordPath is where the record of a root lives under the state
// directory: enrollments/<sha256 of the identity>.json.
func RecordPath(stateDir, root string) string {
	sum := sha256.Sum256([]byte(Identity(root)))
	return filepath.Join(stateDir, recordsDir, hex.EncodeToString(sum[:])+".json")
}

// WriteRecord stores a record atomically: written beside its final path
// and renamed into place, so a reader never sees a partial record.
func WriteRecord(stateDir string, r Record) error {
	if r.Root == "" || r.PolicyDigest == "" || r.WrittenBy == "" {
		return fmt.Errorf("%w: a record needs a root, a policy digest and a writer", model.ErrUsage)
	}
	r.Schema = Schema
	target := RecordPath(stateDir, r.Root)
	if err := os.MkdirAll(filepath.Dir(target), recordsDirMode); err != nil {
		return fmt.Errorf("create the records directory: %w", err)
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("encode the record: %w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(target), ".record-*")
	if err != nil {
		return fmt.Errorf("create the record: %w", err)
	}
	if _, err := temp.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write the record: %w", errors.Join(err, temp.Close(), os.Remove(temp.Name())))
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close the record: %w", errors.Join(err, os.Remove(temp.Name())))
	}
	if err := os.Rename(temp.Name(), target); err != nil {
		return fmt.Errorf("place the record: %w", errors.Join(err, os.Remove(temp.Name())))
	}
	return nil
}

// ReadRecord loads the record of a root; found is false when none exists.
func ReadRecord(stateDir, root string) (Record, bool, error) {
	data, err := os.ReadFile(RecordPath(stateDir, root))
	if errors.Is(err, fs.ErrNotExist) {
		return Record{}, false, nil
	}
	if err != nil {
		return Record{}, false, fmt.Errorf("%w: read the enrollment record: %w", model.ErrEnrollment, err)
	}
	var r Record
	if err := json.Unmarshal(data, &r); err != nil {
		return Record{}, false, fmt.Errorf("%w: the enrollment record is damaged: %w", model.ErrEnrollment, err)
	}
	if r.Schema != Schema || r.Root == "" || r.PolicyDigest == "" {
		return Record{}, false, fmt.Errorf("%w: the enrollment record of %s is incomplete", model.ErrEnrollment, root)
	}
	return r, true, nil
}

// FindRecord looks for the record of dir or of any directory above it, so
// an enrolled repository is recognized even when git cannot discover it
// (A14). The directory must exist; its real path is walked.
func FindRecord(stateDir, dir string) (Record, bool, error) {
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return Record{}, false, fmt.Errorf("%w: resolve %s: %w", model.ErrUsage, dir, err)
	}
	for current := resolved; ; current = filepath.Dir(current) {
		r, found, err := ReadRecord(stateDir, current)
		if err != nil || found {
			return r, found, err
		}
		if filepath.Dir(current) == current {
			return Record{}, false, nil
		}
	}
}

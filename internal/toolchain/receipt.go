package toolchain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// The tool directory is the developer's own: nobody else on the machine
// needs to read or replace what the hook executes.
const (
	dirMode  = 0o700
	fileMode = 0o600
)

// Receipt records what setup installed, so the resolver trusts a tool by
// its recorded size, time and checksum without hashing it on every event.
type Receipt struct {
	Name    string    `json:"name"`
	Version string    `json:"version"`
	Path    string    `json:"path"`
	SHA256  string    `json:"sha256"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
}

func validReceipt(r Receipt) error {
	switch {
	case !toolName.MatchString(r.Name):
		return fmt.Errorf("%w: name %q", ErrReceipt, r.Name)
	case r.Version == "":
		return fmt.Errorf("%w: %s without version", ErrReceipt, r.Name)
	case !filepath.IsAbs(r.Path):
		return fmt.Errorf("%w: %s path %q is not absolute", ErrReceipt, r.Name, r.Path)
	case !sha256Hex.MatchString(r.SHA256):
		return fmt.Errorf("%w: %s sha256 %q", ErrReceipt, r.Name, r.SHA256)
	case r.Size < 0:
		return fmt.Errorf("%w: %s size %d", ErrReceipt, r.Name, r.Size)
	case r.ModTime.IsZero():
		return fmt.Errorf("%w: %s without modification time", ErrReceipt, r.Name)
	}
	return nil
}

// ReceiptPath is the receipt file of one tool.
func ReceiptPath(toolsDir, name string) string {
	return filepath.Join(toolsDir, "receipts", name+".json")
}

// ReadReceipt answers the receipt of a tool; an absent one is no error,
// because a tool nobody installed is simply missing.
func ReadReceipt(toolsDir, name string) (Receipt, bool, error) {
	path := ReceiptPath(toolsDir, name)
	data, err := os.ReadFile(path) //nolint:gosec // the receipt sits in the kit's own tool directory
	if errors.Is(err, fs.ErrNotExist) {
		return Receipt{}, false, nil
	}
	if err != nil {
		return Receipt{}, false, fmt.Errorf("read receipt %s: %w", path, err)
	}
	var r Receipt
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&r); err != nil {
		return Receipt{}, false, fmt.Errorf("%w: %s: %w", ErrReceipt, path, err)
	}
	if r.Name != name {
		return Receipt{}, false, fmt.Errorf("%w: %s names %q", ErrReceipt, path, r.Name)
	}
	if err := validReceipt(r); err != nil {
		return Receipt{}, false, fmt.Errorf("%s: %w", path, err)
	}
	return r, true, nil
}

// WriteReceipt validates before it writes, so a receipt on disk is always
// one ReadReceipt accepts.
func WriteReceipt(toolsDir string, r Receipt) error {
	if toolsDir == "" {
		return ErrNoToolsDir
	}
	if err := validReceipt(r); err != nil {
		return err
	}
	path := ReceiptPath(toolsDir, r.Name)
	if err := os.MkdirAll(filepath.Dir(path), dirMode); err != nil {
		return fmt.Errorf("receipts directory: %w", err)
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("encode receipt %s: %w", r.Name, err)
	}
	return writeAtomic(path, append(data, '\n'), fileMode)
}

// writeAtomic replaces a file by rename, so a hook that reads it while
// setup writes sees the old or the new file, never half of one.
func writeAtomic(path string, data []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".golaw-*")
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	name := tmp.Name()
	err = fillTemp(tmp, data, mode)
	if err == nil {
		err = os.Rename(name, path)
	}
	if err != nil {
		return fmt.Errorf("write %s: %w", path, errors.Join(err, os.Remove(name)))
	}
	return nil
}

// fillTemp writes and closes the temporary file; the close error counts,
// because a full disk can surface only there.
func fillTemp(tmp *os.File, data []byte, mode os.FileMode) error {
	_, err := tmp.Write(data)
	if err == nil {
		err = tmp.Chmod(mode)
	}
	return errors.Join(err, tmp.Close())
}

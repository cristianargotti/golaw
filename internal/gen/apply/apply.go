package apply

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen"
	"github.com/cristianargotti/golaw/internal/source"
)

// The causes of a skipped write, in English for the agent and the command
// output, which name the file and why the engine left it alone.
const (
	CauseChanged = "changed on disk since the hook read it"
	CauseOwned   = "owned file exists"
)

// No file or directory the engine creates is readable by another user
// (M7.10.1); an existing file keeps the bits its owner gave it.
const (
	fileMode fs.FileMode = 0o600
	dirMode  fs.FileMode = 0o700
)

// tempBytes is the random part of a temporary name: 16 hex digits.
const tempBytes = 8

// Skip is a write the engine left undone and why.
type Skip struct {
	Path  string
	Cause string
}

// Report is what one call landed and what it left alone, so the hook and
// the commands say both without reading the tree again.
type Report struct {
	Written []string
	Skipped []Skip
}

// Files lands writes in path order through an os.Root, so no write follows a
// link or a parent step out of root. Each lands, or a delete removes, only
// over the bytes its plan read, through a temporary file renamed over the target; the first failure
// answers what was written before it and leaves every later file untouched.
func Files(root string, writes []gen.Write) (Report, error) {
	r, err := os.OpenRoot(root)
	if err != nil {
		return Report{}, fmt.Errorf("open the repository root: %w", err)
	}
	sorted := slices.Clone(writes)
	slices.SortFunc(sorted, func(a, b gen.Write) int { return strings.Compare(a.Path, b.Path) })
	report, err := landAll(r, sorted)
	return report, errors.Join(err, r.Close())
}

// landAll lands each write in order and stops at the first failure.
func landAll(r *os.Root, writes []gen.Write) (Report, error) {
	var report Report
	for i := range writes {
		landed, cause, err := land(r, &writes[i])
		if err != nil {
			return report, err
		}
		switch {
		case cause != "":
			report.Skipped = append(report.Skipped, Skip{Path: writes[i].Path, Cause: cause})
		case landed:
			report.Written = append(report.Written, writes[i].Path)
		}
	}
	return report, nil
}

// land writes or deletes one file unless the tree moved since the plan read
// it, and answers whether it changed the tree and the cause of a skip.
func land(r *os.Root, w *gen.Write) (bool, string, error) {
	rel, err := source.CleanRel(w.Path)
	if err != nil || rel != w.Path {
		return false, "", fmt.Errorf("%w: %s", ErrOutside, w.Path)
	}
	content, mode, present, err := state(r, rel)
	if err != nil {
		return false, "", err
	}
	if w.Mode == gen.WriteDelete {
		return remove(r, rel, w.Base, content, present)
	}
	switch {
	case present && w.Mode == gen.WriteOwned:
		return false, CauseOwned, nil
	case present && (w.Base == "" || source.ContentHash(content) != w.Base), !present && w.Base != "":
		return false, CauseChanged, nil
	}
	if !present {
		mode = fileMode
	}
	if err := replace(r, rel, w.Content, mode); err != nil {
		return false, "", fmt.Errorf("write %s: %w", rel, err)
	}
	return true, "", nil
}

// remove deletes a file whose bytes still hash to base. A file already gone
// is nothing to do, and one edited since the run read it is kept, so a
// removal never loses an edit.
func remove(r *os.Root, rel, base string, content []byte, present bool) (bool, string, error) {
	switch {
	case !present:
		return false, "", nil
	case source.ContentHash(content) != base:
		return false, CauseChanged, nil
	}
	if err := r.Remove(rel); err != nil {
		return false, "", fmt.Errorf("remove %s: %w", rel, err)
	}
	return true, "", nil
}

// state reads the bytes and permission bits at rel; an absent file is no
// error, any other failure is, because guessing would risk a lost edit.
func state(r *os.Root, rel string) ([]byte, fs.FileMode, bool, error) {
	info, err := r.Stat(rel)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, 0, false, nil
	}
	if err == nil {
		var content []byte
		content, err = r.ReadFile(rel)
		if err == nil {
			return content, info.Mode().Perm(), true, nil
		}
	}
	return nil, 0, false, fmt.Errorf("read %s: %w", rel, err)
}

// discard removes the temporary file a failed write left; a file that was
// never created is no second failure.
func discard(r *os.Root, tmp string, err error) error {
	if removeErr := r.Remove(tmp); removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
		return errors.Join(err, removeErr)
	}
	return err
}

// replace writes content beside rel and renames it over rel, so a reader
// sees the old bytes or the new ones and never a torn file.
func replace(r *os.Root, rel string, content []byte, mode fs.FileMode) error {
	dir := path.Dir(rel)
	if err := r.MkdirAll(dir, dirMode); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	random := make([]byte, tempBytes)
	if _, err := rand.Read(random); err != nil {
		return fmt.Errorf("name a temporary file: %w", err)
	}
	tmp := path.Join(dir, ".golaw-gen-"+hex.EncodeToString(random)+".tmp")
	err := writeTemp(r, tmp, content, mode)
	if err == nil {
		err = r.Rename(tmp, rel)
	}
	if err != nil {
		return discard(r, tmp, fmt.Errorf("replace %s: %w", rel, err))
	}
	return nil
}

// writeTemp creates the temporary file and gives it mode exactly: the umask
// must neither narrow an existing file's bits nor widen a new one's.
func writeTemp(r *os.Root, tmp string, content []byte, mode fs.FileMode) error {
	f, err := r.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return fmt.Errorf("create %s: %w", tmp, err)
	}
	_, writeErr := f.Write(content)
	if err := errors.Join(writeErr, f.Sync(), f.Close()); err != nil {
		return fmt.Errorf("fill %s: %w", tmp, err)
	}
	if err := r.Chmod(tmp, mode); err != nil {
		return fmt.Errorf("chmod %s: %w", tmp, err)
	}
	return nil
}

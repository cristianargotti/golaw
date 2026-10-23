package profile

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/cristianargotti/golaw/internal/policy"
)

// bundleFileMode is the mode of every file a bundle copy writes: data the
// person reads through the kit, under a directory only they can open.
const bundleFileMode = 0o600

// Store copies every regular file of a read bundle but .git into the
// stored directory of its version (policy.StoredStampPath) and makes it
// the current bundle of its name (D3). It answers false when the stored
// copy and the current bundle already held this version with this digest,
// so nothing was written. A stored copy of the version with another digest
// is a moved tag and is refused, because a stored version never changes.
func Store(configDir string, b *Bundle) (bool, error) {
	stored := filepath.Dir(policy.StoredStampPath(configDir, b.Stamp.Name, b.Stamp.Version))
	wroteStore, err := place(b, stored, true)
	if err != nil {
		return false, err
	}
	wroteCurrent, err := place(b, Dir(configDir, b.Stamp.Name), false)
	if err != nil {
		return false, err
	}
	return wroteStore || wroteCurrent, nil
}

// place leaves dir as it is when it already reads as the bundle's version
// and digest and writes the bundle over it otherwise; a stored copy that
// reads as anything else is refused instead of replaced, while a damaged
// copy, which no pin can load, is replaced.
func place(b *Bundle, dir string, stored bool) (bool, error) {
	held, err := Read(dir)
	switch {
	case err == nil && held.Stamp.Version == b.Stamp.Version && held.Data.Digest == b.Data.Digest:
		return false, nil
	case err == nil && stored:
		return false, movedTag(b.Stamp.Name, b.Stamp.Version, b.Data.Digest, held.Data.Digest)
	}
	return true, replaceDir(b.Dir, dir)
}

// replaceDir copies the bundle beside dst and renames it into place; an
// existing dst is moved aside first and removed after, so the name is
// absent for the span of one rename and a reader never sees half a bundle.
func replaceDir(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), profilesMode); err != nil {
		return bundlef(createCause, filepath.Dir(dst), err)
	}
	work, err := os.MkdirTemp(filepath.Dir(dst), ".place-*")
	if err != nil {
		return bundlef("stage %s: %v", dst, err)
	}
	staged, aside := filepath.Join(work, "new"), filepath.Join(work, "old")
	if err := copyBundle(src, staged); err != nil {
		return errors.Join(err, removeDir(work))
	}
	if err := os.Rename(dst, aside); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return errors.Join(bundlef("move %s aside: %v", dst, err), removeDir(work))
	}
	if err := os.Rename(staged, dst); err != nil {
		return errors.Join(bundlef("place %s: %v", dst, err), restore(aside, dst), removeDir(work))
	}
	return removeDir(work)
}

// restore puts back the bundle a failed placement moved aside.
func restore(aside, dst string) error {
	if err := os.Rename(aside, dst); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return bundlef("restore %s: %v", dst, err)
	}
	return nil
}

// copyBundle copies every regular file of src but a .git directory at its
// root into dst, the files the digest covers plus the stamp.
func copyBundle(src, dst string) error {
	//nolint:wrapcheck // every error of the walk is the callback's own bundle error
	return filepath.WalkDir(src, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return bundlef(readCause, path, walkErr)
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return bundlef("relativize %s: %v", path, err)
		}
		target := filepath.Join(dst, rel)
		switch {
		case entry.IsDir() && rel == gitDir:
			return filepath.SkipDir
		case entry.IsDir():
			return makeDir(target)
		case !entry.Type().IsRegular():
			return bundlef("%s is not a regular file", filepath.ToSlash(rel))
		}
		return copyFile(path, target)
	})
}

// makeDir creates one directory of a bundle copy.
func makeDir(dir string) error {
	if err := os.MkdirAll(dir, profilesMode); err != nil {
		return bundlef(createCause, dir, err)
	}
	return nil
}

// copyFile copies one file of a bundle; bundles are small data files.
func copyFile(from, to string) error {
	data, err := os.ReadFile(from) //nolint:gosec // the path is an entry of the bundle being copied
	if err != nil {
		return bundlef(readCause, from, err)
	}
	if err := os.WriteFile(to, data, bundleFileMode); err != nil { //nolint:gosec // the path is under the staging directory of the copy
		return bundlef("write %s: %v", to, err)
	}
	return nil
}

// movedTag is the refusal of a version that came back with another digest.
func movedTag(name, version, fresh, held string) error {
	return bundlef("profile %s %s: the source now holds digest %s and the installed copy holds %s: "+
		"a published version never changes; ask its author for a new version, or run golaw profile remove %s to start over",
		name, version, fresh, held, name)
}

package profile

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/cristianargotti/golaw/internal/policy"
)

// gitDir is the one directory of a clone a bundle never carries.
const gitDir = ".git"

// Digest answers the bundle digest of a directory (D2): DigestPrefix and
// the hex SHA-256 over "<slash path>\n<sha256 hex of the bytes>\n" of every
// regular file but the stamp at the root, sorted by path, a .git directory
// left out. A symbolic link or another irregular entry is ErrBundle, so a
// bundle never points outside itself.
func Digest(root string) (string, error) {
	paths, err := bundleFiles(root)
	if err != nil {
		return "", err
	}
	sum := sha256.New()
	for _, rel := range paths {
		fileSum, err := fileDigest(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return "", err
		}
		_, _ = io.WriteString(sum, rel+"\n"+fileSum+"\n") //nolint:errcheck // a hash write never fails
	}
	return DigestPrefix + hex.EncodeToString(sum.Sum(nil)), nil
}

// bundleFiles lists the files the digest covers, sorted by slash path.
func bundleFiles(root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return bundlef(readCause, path, walkErr)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return bundlef("relativize %s: %v", path, err)
		}
		rel = filepath.ToSlash(rel)
		switch {
		case entry.IsDir() && entry.Name() == gitDir && rel == gitDir:
			return filepath.SkipDir
		case entry.IsDir(), rel == policy.StampFile:
			return nil
		case !entry.Type().IsRegular():
			return bundlef("%s is not a regular file", rel)
		}
		paths = append(paths, rel)
		return nil
	})
	if err != nil {
		return nil, bundlef(readCause, root, err)
	}
	sort.Strings(paths)
	return paths, nil
}

// fileDigest is the hex SHA-256 of one file.
func fileDigest(path string) (string, error) {
	file, err := os.Open(path) //nolint:gosec // the path is an entry of the bundle directory being hashed
	if err != nil {
		return "", bundlef(readCause, path, err)
	}
	defer file.Close()
	sum := sha256.New()
	if _, err := io.Copy(sum, file); err != nil {
		return "", bundlef(readCause, path, err)
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}

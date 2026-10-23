package install

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// notDirIsAbsent holds on a system where Go reads a regular file in the
// place of a directory as nothing there: Windows answers a path under a
// file, or a file opened as a directory, with ERROR_PATH_NOT_FOUND, the
// errno Go names ENOTDIR there, which errors.Is matches to
// fs.ErrNotExist, where Unix answers ENOTDIR.
var notDirIsAbsent = errors.Is(syscall.ENOTDIR, fs.ErrNotExist)

func testOptions(t *testing.T) Options {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	must(t, err)
	options := Options{
		Root:      filepath.Join(base, "repository"),
		ConfigDir: filepath.Join(base, "config"),
		StateDir:  filepath.Join(base, "state"),
		Scope:     "user", Version: "0.1.0",
		PolicyDigest: "policy", ProfileDigest: "",
		ReadRules: []string{"Read(**/*.pem)", "Read(**/*.key)"},
	}
	for _, path := range []string{options.Root, options.ConfigDir, options.StateDir} {
		must(t, os.MkdirAll(path, 0o700))
	}
	return options
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

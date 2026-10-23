package profile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cristianargotti/golaw/internal/policy"
)

// Install clones src, verifies the bundle and makes it the current one of
// its name, keeping every installed version in the store (D3, D9): the
// clone is read whole (the stamp, the digest recomputed, rules.yml decoded
// and validated), its stamp must carry the tag's version, and a version the
// record or the store already holds with another digest is refused. The
// bool reports a version that was already current with the same digest,
// in which case nothing was written. It never reads or writes a golaw.yml.
func Install(ctx context.Context, dirs policy.Dirs, src Source, env []string, now time.Time) (Installed, bool, error) {
	src.URL = localSource(src.URL)
	return installFrom(ctx, dirs, src, env, now, "")
}

// localSource makes a source that names a directory of this machine by a
// relative path absolute, because the clone runs in the profiles directory
// and a refresh may run from any other one; a URL, an ssh address and an
// absolute path stay as the person gave them.
func localSource(url string) string {
	if filepath.IsAbs(url) || strings.Contains(url, "://") {
		return url
	}
	if info, err := os.Stat(url); err != nil || !info.IsDir() {
		return url
	}
	abs, err := filepath.Abs(url)
	if err != nil {
		return url
	}
	return abs
}

// installFrom is Install held to a name when name is set, so the refresh of a
// record never turns into another profile; the clone is removed on every
// path.
func installFrom(ctx context.Context, dirs policy.Dirs, src Source, env []string, now time.Time, name string) (Installed, bool, error) {
	clone, err := Fetch(ctx, dirs.Config, src, env)
	if err != nil {
		return Installed{}, false, err
	}
	installed, already, err := installClone(dirs, src, clone, now, name)
	return installed, already, errors.Join(err, removeDir(clone))
}

// installClone verifies the clone, stores it and moves the record.
func installClone(dirs policy.Dirs, src Source, clone string, now time.Time, name string) (Installed, bool, error) {
	b, err := Read(clone)
	if err != nil {
		return Installed{}, false, err
	}
	if mismatch := matchesSource(b, src, name); mismatch != nil {
		return Installed{}, false, mismatch
	}
	record, _, err := ReadRecord(dirs.State, b.Stamp.Name)
	if err != nil {
		return Installed{}, false, err
	}
	held, known := record.entry(b.Stamp.Version)
	if known && held.Digest != b.Data.Digest {
		return Installed{}, false, movedTag(b.Stamp.Name, b.Stamp.Version, b.Data.Digest, held.Digest)
	}
	wrote, err := Store(dirs.Config, b)
	if err != nil {
		return Installed{}, false, err
	}
	if known && !wrote && record.Current == held.Version {
		return held, true, nil
	}
	installed := Installed{
		Name: b.Stamp.Name, Version: b.Stamp.Version, Digest: b.Data.Digest,
		Source: b.Stamp.Source, URL: src.URL, Tag: src.Tag, InstalledAt: now,
	}
	return installed, false, WriteRecord(dirs.State, record.withCurrent(installed, now))
}

// matchesSource refuses a clone whose stamp is not the tag's version, or,
// for a refresh, not the name of the record.
func matchesSource(b *Bundle, src Source, name string) error {
	if b.Stamp.Version != src.Version {
		return bundlef("profile %s: %s@%s carries version %s in its stamp, not %s",
			b.Stamp.Name, src.URL, src.Tag, b.Stamp.Version, src.Version)
	}
	if name != "" && b.Stamp.Name != name {
		return bundlef("profile %s: %s@%s now holds the profile %s", name, src.URL, src.Tag, b.Stamp.Name)
	}
	return nil
}

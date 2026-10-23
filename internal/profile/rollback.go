package profile

import (
	"path/filepath"
	"time"

	"github.com/cristianargotti/golaw/internal/policy"
)

// Rollback makes the previously current version of a name current again
// (D3): the entry before the current one in the record, read from its
// stored copy and verified against the digest the record holds, then
// copied into place, never linked. A record with one version has nothing
// to roll back to. The pin of golaw.yml is never touched.
func Rollback(dirs policy.Dirs, name string, now time.Time) (Installed, error) {
	record, err := recordFor(dirs.State, name)
	if err != nil {
		return Installed{}, err
	}
	previous, ok := record.previous()
	if !ok {
		return Installed{}, bundlef("profile %s has no earlier version to roll back to: %s is the only one installed", name, record.Current)
	}
	b, err := Read(filepath.Dir(policy.StoredStampPath(dirs.Config, name, previous.Version)))
	if err != nil {
		return Installed{}, bundlef("profile %s %s: the stored copy cannot be read (%v): run golaw profile install %s@%s",
			name, previous.Version, err, previous.URL, previous.Tag)
	}
	if b.Data.Digest != previous.Digest {
		return Installed{}, bundlef("profile %s %s: the stored copy holds digest %s and the record %s: run golaw profile install %s@%s",
			name, previous.Version, b.Data.Digest, previous.Digest, previous.URL, previous.Tag)
	}
	if _, err := Store(dirs.Config, b); err != nil {
		return Installed{}, err
	}
	return previous, WriteRecord(dirs.State, record.withCurrent(previous, now))
}

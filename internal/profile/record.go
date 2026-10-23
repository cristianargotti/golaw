package profile

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/cristianargotti/golaw/internal/model"
)

// The install records live in the state directory beside the enrollment
// records, one JSON file per profile name, readable by its owner only.
const (
	recordsDir    = "profiles"
	recordSchema  = 1
	recordDirMode = 0o700
)

// Record is the install record of one profile name (D3): every installed
// version with where it came from and when, ordered by when each version
// last became current, so the current version is always the last entry
// and the one before it is what a rollback returns to.
type Record struct {
	Schema   int         `json:"golaw"`
	Name     string      `json:"name"`
	Current  string      `json:"current"`  // the version of the current bundle
	Versions []Installed `json:"versions"` // the latest made current last
	Updated  time.Time   `json:"updated"`  // when Current last changed
}

// RecordPath is where the record of a name lives under the state
// directory: profiles/<name>.json.
func RecordPath(stateDir, name string) string {
	return filepath.Join(stateDir, recordsDir, name+".json")
}

// ReadRecord loads the record of a name; found is false when none exists.
// A record that does not decode or breaks its own order is ErrBundle
// naming the file and the way out, never a guess.
func ReadRecord(stateDir, name string) (Record, bool, error) {
	path := RecordPath(stateDir, name)
	data, err := os.ReadFile(path) //nolint:gosec // the path is built under the state directory
	if errors.Is(err, fs.ErrNotExist) {
		return Record{}, false, nil
	}
	if err != nil {
		return Record{}, false, bundlef(readCause, path, err)
	}
	var r Record
	if err := json.Unmarshal(data, &r); err != nil {
		return Record{}, false, damaged(path, name, err)
	}
	if fault := r.fault(name); fault != "" {
		return Record{}, false, damaged(path, name, fault)
	}
	return r, true, nil
}

// WriteRecord stores a record atomically: written beside its final path
// and renamed into place, so a reader never sees a partial record.
func WriteRecord(stateDir string, r Record) error {
	r.Schema = recordSchema
	if fault := r.fault(r.Name); fault != "" {
		return fmt.Errorf("%w: profile record: %s", model.ErrUsage, fault)
	}
	target := RecordPath(stateDir, r.Name)
	if err := os.MkdirAll(filepath.Dir(target), recordDirMode); err != nil {
		return fmt.Errorf("create the profile records directory: %w", err)
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("encode the profile record: %w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(target), ".record-*")
	if err != nil {
		return fmt.Errorf("create the profile record: %w", err)
	}
	if _, err := temp.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write the profile record: %w", errors.Join(err, temp.Close(), os.Remove(temp.Name())))
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close the profile record: %w", errors.Join(err, os.Remove(temp.Name())))
	}
	if err := os.Rename(temp.Name(), target); err != nil {
		return fmt.Errorf("place the profile record: %w", errors.Join(err, os.Remove(temp.Name())))
	}
	return nil
}

// recordFor reads the record a step on an installed name needs: a name
// that is not one is a usage error, and a name without a record is
// ErrBundle naming the install command, since only an install starts one.
func recordFor(stateDir, name string) (Record, error) {
	if err := checkName(name); err != nil {
		return Record{}, err
	}
	record, found, err := ReadRecord(stateDir, name)
	if err != nil {
		return Record{}, err
	}
	if !found {
		return Record{}, bundlef("profile %s has no install record: run golaw profile install <git url>@<tag>", name)
	}
	return record, nil
}

// checkName refuses a profile name that is not lower case kebab case, so
// no path a step builds from it leaves the profiles directory.
func checkName(name string) error {
	if !model.ValidCode(name) {
		return fmt.Errorf("%w: %q is not a profile name (lower case kebab case)", model.ErrUsage, name)
	}
	return nil
}

// entry is the installed version of the record, false when it holds none.
func (r Record) entry(version string) (Installed, bool) {
	at := slices.IndexFunc(r.Versions, func(i Installed) bool { return i.Version == version })
	if at < 0 {
		return Installed{}, false
	}
	return r.Versions[at], true
}

// previous is the version that was current before the current one.
func (r Record) previous() (Installed, bool) {
	if len(r.Versions) < 2 {
		return Installed{}, false
	}
	return r.Versions[len(r.Versions)-2], true
}

// withCurrent is the record with i made current: any earlier entry of its
// version dropped, i appended last, the clock of the change kept.
func (r Record) withCurrent(i Installed, now time.Time) Record {
	versions := slices.DeleteFunc(slices.Clone(r.Versions), func(v Installed) bool { return v.Version == i.Version })
	return Record{Schema: recordSchema, Name: i.Name, Current: i.Version, Versions: append(versions, i), Updated: now}
}

// fault names what breaks the record of a name, empty when nothing does.
func (r Record) fault(name string) string {
	switch {
	case r.Schema != recordSchema:
		return fmt.Sprintf("golaw is %d, not %d", r.Schema, recordSchema)
	case r.Name != name || !model.ValidCode(name):
		return fmt.Sprintf("name is %q, not %q", r.Name, name)
	case len(r.Versions) == 0 || r.Versions[len(r.Versions)-1].Version != r.Current:
		return fmt.Sprintf("current %q is not the last installed version", r.Current)
	}
	return ""
}

// damaged is the error of a record that cannot be trusted.
func damaged(path, name string, cause any) error {
	return bundlef("the install record %s is damaged (%v): run golaw profile remove %s, then install it again", path, cause, name)
}

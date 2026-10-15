package policy

import (
	"context"
	"fmt"

	"github.com/cristianargotti/golaw/internal/model"
)

// Discovery is what the discoverer of a directory answers: the real root
// of the repository and its HEAD commit, empty on an unborn branch.
type Discovery struct {
	Root string
	Head string
}

// Discoverer finds the repository that holds a directory; cmd and the
// hooks hand in the source package's discovery, so this package never
// imports it.
type Discoverer func(ctx context.Context, dir string) (Discovery, error)

// Location is where a run stands: the discovered repository and the
// enrollment record that governs it, nil when the location is positively
// unenrolled.
type Location struct {
	Root   string
	Head   string
	Record *Record
}

// Locate consults the enrollment first (A3) and the repository second,
// and keeps the outcomes of A14 apart: an unenrolled directory answers a
// location without a record (with the discoverer's own error when it is
// outside git); an enrolled repository whose discovery fails answers
// ErrEnrollment naming the record and the cause; an enrolled repository
// answers its root and its record.
func Locate(ctx context.Context, stateDir, dir string, discover Discoverer) (Location, error) {
	found, err := discover(ctx, dir)
	if err != nil {
		r, enrolled, ferr := FindRecord(stateDir, dir)
		if ferr != nil {
			return Location{}, ferr
		}
		if enrolled {
			return Location{Record: &r}, fmt.Errorf("%w: enrolled repository %s: discovery failed: %w", model.ErrEnrollment, r.Root, err)
		}
		return Location{}, err
	}
	r, enrolled, err := ReadRecord(stateDir, found.Root)
	if err != nil {
		return Location{}, err
	}
	loc := Location{Root: found.Root, Head: found.Head}
	if enrolled {
		loc.Record = &r
	}
	return loc, nil
}

package newcode

import (
	"fmt"
	"strings"
	"time"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/surface"
)

const (
	goSuffix   = ".go"
	testSuffix = "_test.go"
	moduleFile = "go.mod"
)

// Universe answers whether a path is a Go file whose lines the pipeline's
// coverage measures (M6.8 item 3). The scope never widens a core law, it
// only describes what the pipeline measures (M6.3.2); generated paths
// without the generated header leave only through the scope's exclusions,
// as the profile lists them.
func Universe(s model.Snapshot, scope surface.Scope) func(path string) bool {
	var nested []string
	for _, file := range s.Files() {
		// The root go.mod has no directory before it, so it never matches.
		if dir, found := strings.CutSuffix(file.Path, "/"+moduleFile); found {
			nested = append(nested, dir+"/")
		}
	}
	return func(name string) bool {
		if !strings.HasSuffix(name, goSuffix) || strings.HasSuffix(name, testSuffix) || !buildable(name) {
			return false
		}
		for _, prefix := range nested {
			if strings.HasPrefix(name, prefix) {
				return false
			}
		}
		class := s.Class(name)
		return (class == model.ClassTracked || class == model.ClassUntracked) && scope.Covered(name)
	}
}

// buildable drops the paths go test ./... never names: a segment that
// starts with a dot or an underscore, and a vendor or testdata directory.
func buildable(name string) bool {
	segments := strings.Split(name, "/")
	for index, segment := range segments {
		if strings.HasPrefix(segment, ".") || strings.HasPrefix(segment, "_") {
			return false
		}
		directory := index < len(segments)-1
		if directory && (segment == "vendor" || segment == "testdata") {
			return false
		}
	}
	return true
}

// Stale reports a report older than a Go file of the universe (M6.8 item
// 6); an equal time is fresh. It reads no clock: the caller passes both
// times, so a test fixes them.
func Stale(reportTime time.Time, s model.Snapshot, universe func(string) bool, modTime func(path string) (time.Time, error)) (bool, error) {
	for _, file := range s.Files() {
		if !universe(file.Path) {
			continue
		}
		changed, err := modTime(file.Path)
		if err != nil {
			return false, fmt.Errorf("%w: the time of %s: %w", ErrReport, file.Path, err)
		}
		if changed.After(reportTime) {
			return true, nil
		}
	}
	return false, nil
}

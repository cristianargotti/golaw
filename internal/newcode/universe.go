package newcode

import (
	"fmt"
	"strings"
	"time"

	"github.com/cristianargotti/golaw/internal/genheader"
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
// only describes what the pipeline measures (M6.3.2). A golaw generated
// file has the class of its path (C-1) and counts like the owned code
// beside it. A third party generated file is left out, as the parity
// number of M6 measured on the gate, unless an inclusion of the scope names
// it, so the local number is never harsher than the gate.
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
		return counted(s, scope, name)
	}
}

// counted reports a Go file of the module whose class the pipeline counts.
func counted(s model.Snapshot, scope surface.Scope, name string) bool {
	class := s.Class(name)
	if class == model.ClassGenerated {
		return (golawHeaded(s, name) || scope.Names(name)) && scope.Covered(name)
	}
	return (class == model.ClassTracked || class == model.ClassUntracked) && scope.Covered(name)
}

// golawHeaded reports a generated file whose first line is the golaw
// header; a file that cannot be read proves nothing.
func golawHeaded(s model.Snapshot, name string) bool {
	content, err := s.Read(name)
	if err != nil {
		return false
	}
	_, ok := genheader.Parse(content)
	return ok
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

package doctor

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/cristianargotti/golaw/internal/install"
)

// idManaged names the diagnostic of the files init manages.
const idManaged = "doctor.managed"

// The states of a managed file beyond ok and unknown, named apart from
// the states of setup so the two diagnostics never share a declaration.
const (
	managedDrift   = "drift"
	managedMissing = "missing"
)

// ManagedFact is the evidence of one file the managed list of golaw.yml
// records: the hash init wrote and the hash the checkout holds now.
type ManagedFact struct {
	Path     string `json:"path"`
	Recorded string `json:"recorded"`
	Current  string `json:"current"`
	Missing  bool   `json:"missing"`
	Error    string `json:"error"`
}

// CollectManaged hashes, in path order, every file the managed list
// records under root, slash separated as golaw.yml writes it; an absent
// file is missing and a file it cannot read keeps its error.
func CollectManaged(root string, recorded map[string]string) []ManagedFact {
	facts := make([]ManagedFact, 0, len(recorded))
	for _, path := range slices.Sorted(maps.Keys(recorded)) {
		fact := ManagedFact{Path: path, Recorded: recorded[path]}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path))) //nolint:gosec // a path the committed policy records under the repository root
		switch {
		case errors.Is(err, fs.ErrNotExist):
			fact.Missing = true
		case err != nil:
			fact.Error = err.Error()
		default:
			fact.Current = install.Digest(data)
		}
		facts = append(facts, fact)
	}
	return facts
}

// AssessManaged reads no file: a file whose hash is the recorded one is
// ok, another hash is drift, which init --update shows and keeps, an
// absent file is missing, which init --update keeps as a local edit too,
// so its line names git restore, and a file doctor could not read is
// unknown.
func AssessManaged(facts []ManagedFact) []Line {
	lines := make([]Line, 0, len(facts))
	for _, fact := range facts {
		line := Line{ID: idManaged, Key: fact.Path, State: stateOK, Code: "managed-matches", Detail: fact.Path}
		switch {
		case fact.Missing:
			line.State, line.Code = managedMissing, "managed-missing"
			line.Detail = fact.Path + " is gone from the checkout, and golaw init --update keeps a deletion as a local edit: restore it with git restore --source=HEAD --staged --worktree -- " + fact.Path
		case fact.Error != "":
			line.State, line.Code, line.Detail = stateUnknown, "managed-unreadable", fact.Path+": "+fact.Error
		case fact.Current != fact.Recorded:
			line.State, line.Code = managedDrift, "managed-drift"
			line.Detail = fact.Path + " differs from the hash init recorded: golaw init --update shows the diff and keeps the edit"
		}
		lines = append(lines, line)
	}
	return lines
}

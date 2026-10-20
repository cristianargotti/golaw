package doctor

import (
	"bytes"
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

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
// records: the hash init wrote and the hash the checkout holds now, and,
// for bytes that hold a CRLF, their hash with each CRLF read as LF, which
// tells a checkout that converted line endings from an edit.
type ManagedFact struct {
	Path      string `json:"path"`
	Recorded  string `json:"recorded"`
	Current   string `json:"current"`
	CurrentLF string `json:"current_lf,omitempty"`
	Missing   bool   `json:"missing"`
	Error     string `json:"error"`
}

// CollectManaged hashes, in path order, every file the managed list
// records under root, slash separated as golaw.yml writes it; an absent
// file is missing and a file it cannot read keeps its error.
func CollectManaged(root string, recorded map[string]string) []ManagedFact {
	facts := make([]ManagedFact, 0, len(recorded))
	for _, path := range slices.Sorted(maps.Keys(recorded)) {
		facts = append(facts, collectManaged(root, path, recorded[path]))
	}
	return facts
}

// collectManaged reads a managed file only as init writes one, a regular
// file reached through directories under root: a link on the way would
// hash bytes the repository does not hold, and a device or a pipe could be
// read without end.
func collectManaged(root, path, recorded string) ManagedFact {
	fact := ManagedFact{Path: path, Recorded: recorded}
	name := root
	parts := strings.Split(path, "/")
	for index, part := range parts {
		name = filepath.Join(name, part)
		info, err := os.Lstat(name)
		if err != nil {
			return managedFailure(fact, err)
		}
		switch last := index == len(parts)-1; {
		case !last && !info.IsDir():
			fact.Error = strings.Join(parts[:index+1], "/") + " is not a directory"
			return fact
		case last && !info.Mode().IsRegular():
			fact.Error = "not a regular file"
			return fact
		}
	}
	data, err := os.ReadFile(name) //nolint:gosec // every component under the repository root was checked above
	if err != nil {
		return managedFailure(fact, err)
	}
	fact.Current = install.Digest(data)
	if folded := bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")); len(folded) != len(data) {
		fact.CurrentLF = install.Digest(folded)
	}
	return fact
}

// managedFailure keeps an absent file as missing and any other failure as
// the error its line names.
func managedFailure(fact ManagedFact, err error) ManagedFact {
	if errors.Is(err, fs.ErrNotExist) {
		fact.Missing = true
		return fact
	}
	fact.Error = err.Error()
	return fact
}

// AssessManaged reads no file: a file whose hash is the recorded one is
// ok, another hash is drift, which init --update shows and keeps, an
// absent file is missing, which init --update keeps as a local edit too,
// so its line names git restore, and a file doctor could not read is
// unknown. A drift of the line endings alone names its cause, because the
// diff init --update shows does not, and git rewrites the file only once
// the attribute says LF and the index forgets the converted copy.
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
		case fact.Current != fact.Recorded && fact.CurrentLF != "" && fact.CurrentLF == fact.Recorded:
			line.State, line.Code = managedDrift, "managed-drift"
			line.Detail = fact.Path + " differs from the hash init recorded only in its CRLF line endings, so golaw init --update keeps it as a local edit: " +
				"add the line * text=auto eol=lf to .gitattributes, run git rm --cached -q -- " + fact.Path +
				", then git restore --source=HEAD --staged --worktree -- " + fact.Path
		case fact.Current != fact.Recorded:
			line.State, line.Code = managedDrift, "managed-drift"
			line.Detail = fact.Path + " differs from the hash init recorded: golaw init --update shows the diff and keeps the edit"
		}
		lines = append(lines, line)
	}
	return lines
}

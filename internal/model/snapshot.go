package model

// unknownName is what every enumeration prints for a value it does not know.
const unknownName = "unknown"

// MaxReadBytes bounds every Snapshot.Read. A file beyond it is answered
// with ErrTooLarge, so a check reports Unmeasured instead of swallowing a
// multi-megabyte blob inside a hook deadline.
const MaxReadBytes = 8 << 20

// FileClass is the one class a snapshot assigns to a path. When several
// descriptions fit, the highest constant wins: a generated test file is
// Generated, an unreadable symlink is Unreadable.
type FileClass int

// The classes of section 5.1, in precedence order.
const (
	ClassUnknown    FileClass = iota // the path is not in the snapshot
	ClassTracked                     // a text file git tracks
	ClassUntracked                   // a text file git would add
	ClassTest                        // a Go test file: keeps the file limit, loses the function limit
	ClassFixture                     // a file under a testdata directory
	ClassGenerated                   // a file with the generated code header
	ClassBinary                      // a NUL byte in its first 8000 bytes
	ClassSymlink                     // a symbolic link; its target is never followed
	ClassGitlink                     // a submodule entry
	ClassUnreadable                  // the snapshot could not read it
)

// String names the class for reports.
func (c FileClass) String() string {
	names := [...]string{
		unknownName, "tracked", "untracked", "test", "fixture",
		"generated", "binary", "symlink", "gitlink", "unreadable",
	}
	if c < 0 || int(c) >= len(names) {
		return unknownName
	}
	return names[c]
}

// IsText reports whether a check may read the file as text: every class up
// to Generated; binary, link, submodule and unreadable paths are skipped.
func (c FileClass) IsText() bool { return c >= ClassTracked && c <= ClassGenerated }

// File is one path of the snapshot universe.
type File struct {
	Path  string    // slash separated, relative to the snapshot root, never absolute
	Class FileClass // the class the snapshot assigned
	Size  int64     // bytes, as the snapshot knows them before any read
}

// ChangeOp is the operation a staged change performs.
type ChangeOp int

// The operations of a staged change, the ACMR filter of git plus a plain
// deletion; a rename carries both paths.
const (
	ChangeAdd ChangeOp = iota + 1
	ChangeModify
	ChangeDelete
	ChangeRename
)

// String names the operation for reports.
func (op ChangeOp) String() string {
	switch op {
	case ChangeAdd:
		return "add"
	case ChangeModify:
		return "modify"
	case ChangeDelete:
		return "delete"
	case ChangeRename:
		return "rename"
	default:
		return unknownName
	}
}

// Change is one staged difference between HEAD and the index.
type Change struct {
	Op      ChangeOp
	OldPath string // the path before the change, empty for an addition
	NewPath string // the path after the change, empty for a deletion
	Mode    string // git tree mode of the new path: 100644, 100755, 120000 or 160000
	Before  string // object id of the old blob, empty for an addition
	After   string // object id of the new blob, empty for a deletion
}

// Snapshot is the read interface source implements and every check reads
// through. It is frozen: Files answers the same universe on every call,
// Read answers the same bytes, and a hook overlay (candidates that do not
// exist on disk yet) is already folded in. Read returns ErrNotInSnapshot,
// ErrUnreadable or ErrTooLarge, never a partial file.
type Snapshot interface {
	Files() []File
	Read(path string) ([]byte, error)
	Class(path string) FileClass
}

// Selector is a snapshot that answers part of its universe without
// classifying the rest, so a rule that reads one directory never makes a
// hook read the whole tree.
type Selector interface {
	// Select answers the files whose paths match, in path order.
	Select(match func(path string) bool) []File
	// Any reports whether a path of the universe matches, without
	// reading a file.
	Any(match func(path string) bool) bool
}

// Select answers the files of a snapshot whose paths match, in path order,
// through the snapshot's own Select when it has one.
func Select(s Snapshot, match func(path string) bool) []File {
	if selector, ok := s.(Selector); ok {
		return selector.Select(match)
	}
	var out []File
	for _, file := range s.Files() {
		if match(file.Path) {
			out = append(out, file)
		}
	}
	return out
}

// Any reports whether a path of a snapshot matches, through the
// snapshot's own Any when it has one.
func Any(s Snapshot, match func(path string) bool) bool {
	if selector, ok := s.(Selector); ok {
		return selector.Any(match)
	}
	for _, file := range s.Files() {
		if match(file.Path) {
			return true
		}
	}
	return false
}

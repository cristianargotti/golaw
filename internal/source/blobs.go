package source

import (
	"context"
	"fmt"
	"strings"
)

// gitlinkMode is the mode git gives a submodule.
const gitlinkMode = "160000"

// noRenames has every diff of the kit read a rename as its two paths, a
// deletion and an addition, so each is judged where it stands.
const noRenames = "--no-renames"

// rawFields are the fields of a record of git diff-tree before its path:
// the two modes, the two objects and the status.
const rawFields = 5

// The names of the empty tree in the two object formats git knows; git
// answers either without holding it as an object.
const (
	emptyTreeSHA1   = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
	emptyTreeSHA256 = "6ef19b41225c5369f1c104d45d8d85efa9b057b53b14b4b9b939dd74decc5321"
)

// Moved answers the paths that differ between two commits with the blob
// the earlier one held at each, empty where it held none, asking git
// diff-tree once; a submodule moved to another revision is left out, as
// change detection excludes it. An empty name is the HEAD of an unborn
// branch, read as the empty tree, so a first commit names every path it
// holds without an earlier blob and an orphan checkout every path of the
// commit it left with the blob that commit holds.
func Moved(ctx context.Context, repo Repo, from, to string) (map[string]string, error) {
	from, to, err := treeEnds(from, to)
	if err != nil {
		return nil, err
	}
	out, err := git(ctx, repo.Root, "diff-tree", "-r", "-z", noRenames, from, to)
	if err != nil {
		return nil, err
	}
	records := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	moved := map[string]string{}
	for i := 0; i+1 < len(records); i += 2 {
		fields := strings.Fields(strings.TrimPrefix(records[i], ":"))
		if len(fields) != rawFields {
			return nil, fmt.Errorf("%w: git diff-tree printed %q", ErrGit, records[i])
		}
		if fields[0] == gitlinkMode && fields[1] == gitlinkMode {
			continue
		}
		blob := fields[2]
		if fields[0] == gitlinkMode || strings.Trim(blob, "0") == "" {
			blob = ""
		}
		moved[records[i+1]] = blob
	}
	return moved, nil
}

// treeEnds puts the empty tree of the object format the other name belongs
// to in place of an empty name, the format a name of that length is in.
func treeEnds(from, to string) (string, string, error) {
	switch {
	case from == "":
		from = emptyTree(to)
	case to == "":
		to = emptyTree(from)
	}
	if from == "" || to == "" {
		return "", "", fmt.Errorf("%w: no object format names a commit %q", ErrGit, from+to)
	}
	return from, to, nil
}

// emptyTree is the name of the empty tree in the object format of a name of
// this length, empty for a length no format has.
func emptyTree(name string) string {
	switch len(name) {
	case len(emptyTreeSHA1):
		return emptyTreeSHA1
	case len(emptyTreeSHA256):
		return emptyTreeSHA256
	default:
		return ""
	}
}

// HashObjects answers the blob git would store for the bytes each path
// holds now, through the conversions its attributes and configuration
// name, the end of line rules and the filters, asking git hash-object
// --stdin-paths once for all of them: it writes no object and takes no
// lock. A name one line of input cannot carry is left out, so a caller
// reads it as unknown.
func HashObjects(ctx context.Context, repo Repo, paths []string) (map[string]string, error) {
	var input []byte
	asked := make([]string, 0, len(paths))
	for _, path := range paths {
		if path == "" || strings.ContainsAny(path, "\n\r") || path[0] == '"' {
			continue
		}
		asked = append(asked, path)
		input = append(input, path+"\n"...)
	}
	hashes := make(map[string]string, len(asked))
	if len(asked) == 0 {
		return hashes, nil
	}
	out, err := gitInput(ctx, repo.Root, input, "hash-object", "--stdin-paths")
	if err != nil {
		return nil, err
	}
	names := strings.Fields(string(out))
	if len(names) != len(asked) {
		return nil, fmt.Errorf("%w: git hash-object named %d of %d paths", ErrGit, len(names), len(asked))
	}
	for i, path := range asked {
		hashes[path] = names[i]
	}
	return hashes, nil
}

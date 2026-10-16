package source

import (
	"context"
	"fmt"
	"strings"
)

// gitlinkMode is the mode git gives a submodule.
const gitlinkMode = "160000"

// rawFields are the fields of a record of git diff-tree before its path:
// the two modes, the two objects and the status.
const rawFields = 5

// Moved answers the paths that differ between two commits with the blob
// the earlier one held at each, empty where it held none, asking git
// diff-tree once; a submodule moved to another revision is left out, as
// change detection excludes it.
func Moved(ctx context.Context, repo Repo, from, to string) (map[string]string, error) {
	out, err := git(ctx, repo.Root, "diff-tree", "-r", "-z", "--no-renames", from, to)
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

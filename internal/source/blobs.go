package source

import (
	"context"
	"fmt"
	"strings"
)

// Blobs answers the blob a commit holds at each of the paths, asking git
// cat-file --batch-check once for all of them. A path the commit holds as
// no file, a tree or a submodule, and a name one line of input cannot
// carry are left out, so a caller reads them as absent from the commit.
func Blobs(ctx context.Context, repo Repo, commit string, paths []string) (map[string]string, error) {
	var input []byte
	asked := make([]string, 0, len(paths))
	for _, path := range paths {
		if commit == "" || strings.ContainsAny(path, "\n\r") {
			continue
		}
		asked = append(asked, path)
		input = append(input, commit+":"+path+"\n"...)
	}
	blobs := make(map[string]string, len(asked))
	if len(asked) == 0 {
		return blobs, nil
	}
	out, err := gitInput(ctx, repo.Root, input, "cat-file", "--batch-check")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	for i, path := range asked {
		if i >= len(lines) {
			break
		}
		if fields := strings.Fields(lines[i]); len(fields) == 3 && fields[1] == "blob" {
			blobs[path] = fields[0]
		}
	}
	return blobs, nil
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

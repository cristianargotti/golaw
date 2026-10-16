package source

import (
	"context"
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

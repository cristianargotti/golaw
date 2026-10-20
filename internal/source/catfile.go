package source

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
)

// The object type a snapshot reads bytes from, and the word git cat-file
// prints for an object id the repository does not hold.
const (
	blobKind    = "blob"
	missingKind = "missing"
)

// catFile is the plumbing that shows objects.
const catFile = "cat-file"

// describedFields are the fields git cat-file prints for an object it
// holds: the id, the type and the size.
const describedFields = 3

// objectInfo is what git cat-file --batch-check answered for one object id:
// its type, missingKind when the repository holds no such object, and its
// size in bytes.
type objectInfo struct {
	kind string
	size int64
}

// objectInfos asks git cat-file --batch-check, once, for the type and the
// size of every id, so no blob is read before its size is known and an
// object the repository lacks is an answer, never a failed command.
func objectInfos(ctx context.Context, root string, env, ids []string) (map[string]objectInfo, error) {
	out, err := gitWith(ctx, root, env, idLines(ids), catFile, "--batch-check", "--buffer")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	if len(lines) != len(ids) {
		return nil, fmt.Errorf("%w: git cat-file described %d of %d objects", ErrGit, len(lines), len(ids))
	}
	infos := make(map[string]objectInfo, len(ids))
	for i, line := range lines {
		info, err := describedAs(line, ids[i])
		if err != nil {
			return nil, err
		}
		infos[ids[i]] = info
	}
	return infos, nil
}

// describedAs reads the line git cat-file prints about one id: the id, its
// type and its size, or the id followed by missing.
func describedAs(line, id string) (objectInfo, error) {
	fields := strings.Fields(line)
	switch {
	case len(fields) == 2 && fields[0] == id && fields[1] == missingKind:
		return objectInfo{kind: missingKind}, nil
	case len(fields) == describedFields && fields[0] == id:
		size, err := strconv.ParseInt(fields[2], 10, 64)
		if err == nil && size >= 0 {
			return objectInfo{kind: fields[1], size: size}, nil
		}
	}
	return objectInfo{}, fmt.Errorf("%w: git cat-file printed %q for %s", ErrGit, line, id)
}

// readBlobs reads the blobs of the ids with one git cat-file --batch: each
// answer is the line that describes the blob, its bytes, then a newline.
// The caller asks only for blobs it knows fit the read bound.
func readBlobs(ctx context.Context, root string, env, ids []string) (map[string][]byte, error) {
	out, err := gitWith(ctx, root, env, idLines(ids), catFile, "--batch", "--buffer")
	if err != nil {
		return nil, err
	}
	contents := make(map[string][]byte, len(ids))
	for _, id := range ids {
		header, rest, _ := bytes.Cut(out, []byte{'\n'})
		info, err := describedAs(string(header), id)
		if err != nil || info.kind != blobKind || info.size >= int64(len(rest)) || rest[info.size] != '\n' {
			return nil, fmt.Errorf("%w: git cat-file answered %q for the blob %s", ErrGit, header, id)
		}
		contents[id], out = rest[:info.size:info.size], rest[info.size+1:]
	}
	return contents, nil
}

// idLines is the standard input of a cat-file batch: one id per line.
func idLines(ids []string) []byte {
	return []byte(strings.Join(ids, "\n") + "\n")
}

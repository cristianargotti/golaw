package source

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// Conflict is what the merge git makes of two commits by itself holds at a
// path beside what each commit holds there, as bytes: Merged is the bytes
// of the merge, a conflict git cannot resolve in its markers, Theirs the
// bytes of the merged commit and Ours those of the commit merged into; each
// is nil where the merge or the commit holds no blob there, or one over
// model.MaxReadBytes.
type Conflict struct {
	Merged []byte
	Theirs []byte
	Ours   []byte
}

// Conflicts answers, for a merge of the commit theirs into the commit ours
// with each strategy option passed as -X <option>, the Conflict of each
// path: git merge-tree --write-tree makes the merge once and git cat-file
// --batch reads the three blobs of every path in one more question, with the
// temporary object directory of AutomaticMergeWith, so no object reaches
// the repository. A name one line of input cannot carry is left out, so a
// caller reads it as unknown.
func Conflicts(ctx context.Context, repo Repo, ours, theirs string, options, paths []string) (map[string]Conflict, error) {
	var conflicts map[string]Conflict
	err := withScratch(repo, func(env []string) error {
		tree, err := mergeTree(ctx, repo.Root, env, ours, theirs, options)
		if err == nil {
			conflicts, err = conflictBytes(ctx, repo.Root, env, tree, theirs, ours, paths)
		}
		return err
	})
	return conflicts, err
}

// conflictBytes reads, with one git cat-file --batch, the blob the tree of
// a merge, the commit merged and the commit merged into hold at each path,
// in that order.
func conflictBytes(ctx context.Context, dir string, env []string, tree, theirs, ours string, paths []string) (map[string]Conflict, error) {
	revisions := []string{tree, theirs, ours}
	var input []byte
	asked := make([]string, 0, len(paths))
	for _, path := range paths {
		if path == "" || strings.ContainsAny(path, "\n\r") {
			continue
		}
		asked = append(asked, path)
		for _, revision := range revisions {
			input = append(input, revision+":"+path+"\n"...)
		}
	}
	conflicts := make(map[string]Conflict, len(asked))
	if len(asked) == 0 {
		return conflicts, nil
	}
	out, err := gitWith(ctx, dir, env, input, "cat-file", "--batch")
	if err != nil {
		return nil, err
	}
	blobs, err := batchBlobs(out, len(revisions)*len(asked))
	if err != nil {
		return nil, err
	}
	for i, path := range asked {
		at := len(revisions) * i
		conflicts[path] = Conflict{Merged: blobs[at], Theirs: blobs[at+1], Ours: blobs[at+2]}
	}
	return conflicts, nil
}

// batchBlobs reads count answers of git cat-file --batch, each a header
// "<name> <type> <size>" followed by the object and a line feed, or one
// line that ends in missing or ambiguous: a blob within model.MaxReadBytes
// is its bytes, any other answer nil.
func batchBlobs(out []byte, count int) ([][]byte, error) {
	blobs := make([][]byte, 0, count)
	for len(blobs) < count {
		header, rest, found := bytes.Cut(out, []byte("\n"))
		if !found {
			return nil, catFileError(out)
		}
		if bytes.HasSuffix(header, []byte(" missing")) || bytes.HasSuffix(header, []byte(" ambiguous")) {
			blobs, out = append(blobs, nil), rest
			continue
		}
		kind, size, err := batchHeader(header, len(rest))
		if err != nil {
			return nil, err
		}
		var blob []byte
		if kind == "blob" && size <= model.MaxReadBytes {
			blob = rest[:size]
		}
		blobs, out = append(blobs, blob), rest[size+1:]
	}
	return blobs, nil
}

// headerFields are the fields of the header git cat-file --batch prints
// before an object: its name, its type and its size.
const headerFields = 3

// batchHeader reads the type and the size a header of git cat-file --batch
// names, when the answer after it holds that many bytes and a line feed.
func batchHeader(header []byte, held int) (string, int, error) {
	fields := strings.Fields(string(header))
	if len(fields) != headerFields {
		return "", 0, catFileError(header)
	}
	size, err := strconv.Atoi(fields[2])
	if err != nil || size < 0 || size >= held {
		return "", 0, catFileError(header)
	}
	return fields[1], size, nil
}

// catFileError is an answer of git cat-file the kit cannot read.
func catFileError(text []byte) error {
	return fmt.Errorf("%w: git cat-file printed %q", ErrGit, text)
}

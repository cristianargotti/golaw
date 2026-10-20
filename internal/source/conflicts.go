package source

import (
	"bytes"
	"context"
	"fmt"
	"slices"
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
// path: git merge-tree --write-tree makes the merge once, git cat-file
// --batch-check describes the three blobs of every path in one more
// question and git cat-file --batch reads those within the read bound in a
// last one, with the temporary object directory of AutomaticMergeWith, so
// no object reaches the repository. A name one line of input cannot carry
// is left out, so a caller reads it as unknown.
func Conflicts(ctx context.Context, repo Repo, ours, theirs string, options, paths []string) (map[string]Conflict, error) {
	var conflicts map[string]Conflict
	err := withScratch(repo, func(env []string) error {
		tree, err := mergeTree(ctx, repo.Root, env, ours, theirs, options)
		if err != nil {
			return err
		}
		read, err := revisionBytes(ctx, repo.Root, env, []string{tree, theirs, ours}, paths)
		if err != nil {
			return err
		}
		conflicts = make(map[string]Conflict, len(read))
		for path, blobs := range read {
			conflicts[path] = Conflict{Merged: blobs[0], Theirs: blobs[1], Ours: blobs[2]}
		}
		return nil
	})
	return conflicts, err
}

// The conflict styles that write the base of a conflict between its
// markers, which a command picks for a conflict it writes again, through
// git checkout --conflict or a git -c merge.conflictStyle of its own.
const (
	styleDiff3  = "diff3"
	styleZdiff3 = "zdiff3"
)

// StyledConflicts answers, for a merge of the commit theirs into the commit
// ours that git makes by itself, the bytes the merge holds at each path in
// the diff3 style and in the zdiff3 style, in that order, nil where it
// holds no blob or one over model.MaxReadBytes: git merge-tree --write-tree
// makes the merge once in each style, and git cat-file sizes and reads the
// blobs of both, with the temporary object directory of
// AutomaticMergeWith. A name one line of input cannot carry is left out.
func StyledConflicts(ctx context.Context, repo Repo, ours, theirs string, paths []string) (map[string][][]byte, error) {
	var styled map[string][][]byte
	err := withScratch(repo, func(env []string) error {
		trees := make([]string, 0, 2)
		for _, style := range []string{styleDiff3, styleZdiff3} {
			tree, err := mergeTree(ctx, repo.Root, env, ours, theirs, nil, "-c", "merge.conflictStyle="+style)
			if err != nil {
				return err
			}
			trees = append(trees, tree)
		}
		var err error
		styled, err = revisionBytes(ctx, repo.Root, env, trees, paths)
		return err
	})
	return styled, err
}

// revisionBytes reads the blob each revision holds at each path, in the
// order of the revisions. The sizes come first because git cat-file
// --batch prints a whole object before its reader can drop it, so a blob
// over the bound is never printed at all.
func revisionBytes(ctx context.Context, dir string, env, revisions, paths []string) (map[string][][]byte, error) {
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
	read := make(map[string][][]byte, len(asked))
	if len(asked) == 0 {
		return read, nil
	}
	names, err := boundedNames(ctx, dir, env, input, len(revisions)*len(asked))
	if err != nil {
		return nil, err
	}
	blobs, err := blobsByName(ctx, dir, env, names)
	if err != nil {
		return nil, err
	}
	for i, path := range asked {
		read[path] = make([][]byte, len(revisions))
		for j := range revisions {
			read[path][j] = blobs[names[len(revisions)*i+j]]
		}
	}
	return read, nil
}

// boundedNames are the objects git cat-file --batch-check names for count
// lines of input, one for each line: the name of a blob within
// model.MaxReadBytes, and empty for any other answer, so only such a blob
// is ever read.
func boundedNames(ctx context.Context, dir string, env []string, input []byte, count int) ([]string, error) {
	out, err := catFileBatch(ctx, dir, env, input, "--batch-check")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	if len(lines) != count {
		return nil, catFileError(out)
	}
	names := make([]string, count)
	for i, line := range lines {
		if strings.HasSuffix(line, " missing") || strings.HasSuffix(line, " ambiguous") {
			continue
		}
		name, kind, size, err := objectLine([]byte(line))
		if err != nil {
			return nil, err
		}
		if heldBlob(kind, size) {
			names[i] = name
		}
	}
	return names, nil
}

// blobsByName reads the bytes of each blob named, empty names left out,
// with one git cat-file --batch, and none when no name is left.
func blobsByName(ctx context.Context, dir string, env, names []string) (map[string][]byte, error) {
	var input []byte
	var wanted []string
	for _, name := range names {
		if name != "" && !slices.Contains(wanted, name) {
			wanted = append(wanted, name)
			input = append(input, name+"\n"...)
		}
	}
	blobs := make(map[string][]byte, len(wanted))
	if len(wanted) == 0 {
		return blobs, nil
	}
	out, err := catFileBatch(ctx, dir, env, input, "--batch")
	if err != nil {
		return nil, err
	}
	read, err := batchBlobs(out, len(wanted))
	if err != nil {
		return nil, err
	}
	for i, name := range wanted {
		blobs[name] = read[i]
	}
	return blobs, nil
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
		_, kind, size, err := objectLine(header)
		if err != nil || size >= len(rest) {
			return nil, catFileError(header)
		}
		var blob []byte
		if heldBlob(kind, size) {
			blob = rest[:size]
		}
		blobs, out = append(blobs, blob), rest[size+1:]
	}
	return blobs, nil
}

// headerFields are the fields of the line git cat-file prints to describe
// an object: its name, its type and its size.
const headerFields = 3

// objectLine reads the name, the type and the size a line of git cat-file
// describes an object with.
func objectLine(line []byte) (string, string, int, error) {
	fields := strings.Fields(string(line))
	if len(fields) != headerFields {
		return "", "", 0, catFileError(line)
	}
	size, err := strconv.Atoi(fields[2])
	if err != nil || size < 0 {
		return "", "", 0, catFileError(line)
	}
	return fields[0], fields[1], size, nil
}

// heldBlob reports whether an object git cat-file describes is a blob the
// read bound lets the kit hold.
func heldBlob(kind string, size int) bool {
	return kind == "blob" && size <= model.MaxReadBytes
}

// catFileBatch runs git cat-file in one batch mode over the lines of input:
// the conflict read asks it twice, once to size the blobs and once to read
// those the bound lets it hold.
func catFileBatch(ctx context.Context, dir string, env []string, input []byte, mode string) ([]byte, error) {
	return gitWith(ctx, dir, env, input, "cat-file", mode)
}

// catFileError is an answer of git cat-file the kit cannot read.
func catFileError(text []byte) error {
	return fmt.Errorf("%w: git cat-file printed %q", ErrGit, text)
}

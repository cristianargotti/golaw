package policy

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// The names of the git answers about golaw.yml: the name git resolves to
// the file HEAD holds, the plumbing that shows objects, the type of a file
// and the fields of the header cat-file --batch prints before a blob.
const (
	headName     = "HEAD:" + FileName
	catFile      = "cat-file"
	blobType     = "blob"
	headerFields = 3
)

// reads keeps the blob at HEAD one load read, so a load asks for it once
// and the index is compared with it by id instead of being read again.
type reads struct {
	root string
	head *blobRead
}

// blobRead is one blob as git answered it, or why it could not.
type blobRead struct {
	data  []byte
	oid   string
	found bool
	err   error
}

// atHead reads the golaw.yml HEAD holds with one cat-file --batch, which
// answers the blob by name or says it is missing; HEAD must exist, because
// an unborn HEAD is missing too.
func (r *reads) atHead(ctx context.Context) blobRead {
	if r.head == nil {
		out, err := plumbingInput(ctx, r.root, []byte(headName+"\n"), catFile, "--batch")
		got := blobRead{err: err}
		if err == nil {
			got.data, got.oid, got.found, got.err = batchBlob(out)
		}
		r.head = &got
	}
	return *r.head
}

// staged lists the index entry of golaw.yml and answers the blob it
// holds: the blob read at HEAD when the index holds that same blob,
// otherwise the staged blob read by its id. An entry a merge left
// unresolved is ErrPolicy (ReadIndex).
func (r *reads) staged(ctx context.Context) blobRead {
	listing, err := plumbing(ctx, r.root, "ls-files", "-z", "--stage", "--", FileName)
	if err != nil {
		return blobRead{err: err}
	}
	if unmerged(listing) {
		return blobRead{err: fmt.Errorf("%w: %s is unresolved in the index: finish the merge before the policy is judged", model.ErrPolicy, FileName)}
	}
	line, _, _ := strings.Cut(strings.TrimRight(string(listing), "\x00"), "\x00")
	if line == "" {
		return blobRead{}
	}
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return blobRead{err: fmt.Errorf("%w: unexpected listing %q", model.ErrPolicy, line)}
	}
	got := blobRead{oid: fields[1], found: true}
	if head := r.head; head != nil && head.err == nil && head.found && head.oid == got.oid {
		got.data = head.data
		return got
	}
	got.data, got.err = plumbing(ctx, r.root, catFile, blobType, got.oid)
	return got
}

// batchBlob parses the answer of git cat-file --batch to the name of the
// golaw.yml HEAD holds: the header with the id, the type and the size,
// then the content; or the name followed by missing when the name
// resolves to nothing.
func batchBlob(out []byte) ([]byte, string, bool, error) {
	header, rest, _ := bytes.Cut(out, []byte{'\n'})
	fields := strings.Fields(string(header))
	switch {
	case len(fields) == 2 && fields[0] == headName && fields[1] == "missing":
		return nil, "", false, nil
	case len(fields) != headerFields:
		return nil, "", false, fmt.Errorf("%w: git cat-file answered %q for %s", model.ErrPolicy, header, headName)
	case fields[1] != blobType:
		return nil, "", false, fmt.Errorf("%w: %s is a %s, not a file", model.ErrPolicy, headName, fields[1])
	}
	size, err := strconv.Atoi(fields[2])
	if err != nil || size < 0 || size > len(rest) {
		return nil, "", false, fmt.Errorf("%w: git cat-file answered a short blob for %s", model.ErrPolicy, headName)
	}
	return rest[:size], fields[0], true, nil
}

// plumbingInput is plumbing with the bytes a command reads on stdin.
func plumbingInput(ctx context.Context, root string, input []byte, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // the arguments are the kit's own plumbing
	cmd.Dir = root
	cmd.Stdin = bytes.NewReader(input)
	return output(cmd, args)
}

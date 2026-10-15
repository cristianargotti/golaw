package policy

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// ReadHead reads golaw.yml as HEAD holds it: the policy in force (A2).
// The object id is listed first and the blob read by that id, so what is
// read is exactly what was listed; found is false when HEAD holds no such
// path. HEAD must exist; Load consults the bootstrap record otherwise.
func ReadHead(ctx context.Context, root string) ([]byte, bool, error) {
	listing, err := plumbing(ctx, root, "ls-tree", "-z", "HEAD", "--", FileName)
	if err != nil {
		return nil, false, err
	}
	return readListed(ctx, root, listing, 2)
}

// ReadIndex reads golaw.yml as the index holds it: the proposed next
// policy a staged change carries; found is false when nothing is staged
// at that path.
func ReadIndex(ctx context.Context, root string) ([]byte, bool, error) {
	listing, err := plumbing(ctx, root, "ls-files", "-z", "--stage", "--", FileName)
	if err != nil {
		return nil, false, err
	}
	return readListed(ctx, root, listing, 1)
}

// readListed takes the object id at a field of a listing line and reads
// that blob; an empty listing means the path is absent.
func readListed(ctx context.Context, root string, listing []byte, field int) ([]byte, bool, error) {
	line, _, _ := strings.Cut(strings.TrimRight(string(listing), "\x00"), "\x00")
	if line == "" {
		return nil, false, nil
	}
	fields := strings.Fields(line)
	if len(fields) <= field {
		return nil, false, fmt.Errorf("%w: unexpected listing %q", model.ErrPolicy, line)
	}
	content, err := plumbing(ctx, root, "cat-file", "blob", fields[field])
	if err != nil {
		return nil, false, err
	}
	return content, true, nil
}

// plumbing runs one git command under root and answers its stdout; any
// failure is an ErrPolicy naming the command, because without it the
// policy in force cannot be established.
func plumbing(ctx context.Context, root string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // the arguments are the kit's own plumbing
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		reason, _, _ := strings.Cut(strings.TrimSpace(stderr.String()), "\n")
		if reason == "" {
			reason = err.Error()
		}
		return nil, fmt.Errorf("%w: git %s: %s", model.ErrPolicy, strings.Join(args, " "), reason)
	}
	return out, nil
}

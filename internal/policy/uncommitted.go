package policy

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/cristianargotti/golaw/internal/model"
)

// maxPolicyBytes bounds the golaw.yml of a checkout the hook reads.
const maxPolicyBytes = 1 << 20

// uncommitted is the policy in force of an enrolled repository whose HEAD
// holds no golaw.yml yet: the golaw.yml the index holds, or the one the
// checkout holds when nothing is staged, only when its bytes mean the
// policy the record enrolled. Anything else asks for the commit, because
// a policy an agent can edit never weakens the one in force.
func (l *Loaded) uncommitted(ctx context.Context) ([]byte, error) {
	staged := l.reads().staged(ctx)
	if staged.err != nil {
		return nil, staged.err
	}
	data, origin := staged.data, OriginStaged
	if !staged.found {
		var read bool
		data, read = checkoutPolicy(l.Root)
		origin = OriginCheckout
		if !read {
			return nil, l.commitFirst()
		}
	}
	if f, err := Decode(data); err != nil || Digest(f) != l.Record.PolicyDigest {
		return nil, l.commitFirst()
	}
	l.Origin = origin
	return data, nil
}

// checkoutPolicy reads the golaw.yml of the checkout when it is a regular
// file within the bound, never through a link, because a link is what a
// commit would hold in its place.
func checkoutPolicy(root string) ([]byte, bool) {
	path := filepath.Join(root, FileName)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, false
	}
	file, err := os.Open(path) //nolint:gosec // the policy file of the enrolled root
	if err != nil {
		return nil, false
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxPolicyBytes+1))
	return data, err == nil && len(data) <= maxPolicyBytes
}

// commitFirst is the refusal of every other state before the first commit,
// in words the agent acts on.
func (l *Loaded) commitFirst() error {
	return fmt.Errorf("%w: enrolled repository %s has no %s at HEAD (%s) and holds no %s that is the policy its record enrolled: ask the person to commit %s: %w",
		model.ErrEnrollment, l.Root, FileName, model.EnrollmentMissing, FileName, FileName, ErrNotAtHead)
}

package policy_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

var errOutside = errors.New("not a git repository")

// fakeDiscover answers root for every directory, or the error when root is empty.
func fakeDiscover(root string) policy.Discoverer {
	return func(_ context.Context, _ string) (policy.Discovery, error) {
		if root == "" {
			return policy.Discovery{}, errOutside
		}
		return policy.Discovery{Root: root, Head: "abc"}, nil
	}
}

func TestLocate(t *testing.T) {
	t.Parallel()
	state := filepath.Join(t.TempDir(), "state")
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, "sub/x.txt", "x\n")
	sub := filepath.Join(root, "sub")
	loc, err := policy.Locate(t.Context(), state, sub, fakeDiscover(root))
	if err != nil || loc.Root != root || loc.Head != "abc" || loc.Record != nil {
		t.Errorf("unenrolled repository = %+v, %v", loc, err)
	}
	_, err = policy.Locate(t.Context(), state, sub, fakeDiscover(""))
	if !errors.Is(err, errOutside) {
		t.Errorf("unenrolled directory outside git keeps the discoverer's error: %v", err)
	}
	writeRecord(t, state, record(root))
	loc, err = policy.Locate(t.Context(), state, sub, fakeDiscover(root))
	if err != nil || loc.Record == nil || loc.Record.Root != root {
		t.Errorf("enrolled repository = %+v, %v", loc, err)
	}
	loc, err = policy.Locate(t.Context(), state, sub, fakeDiscover(""))
	if !errors.Is(err, model.ErrEnrollment) || !errors.Is(err, errOutside) || loc.Record == nil {
		t.Errorf("enrolled repository whose discovery fails = %+v, %v", loc, err)
	}
	_, err = policy.Locate(t.Context(), state, filepath.Join(root, "absent"), fakeDiscover(""))
	if !errors.Is(err, model.ErrUsage) {
		t.Errorf("an absent directory: %v", err)
	}
	write(t, filepath.Dir(policy.RecordPath(state, root)), filepath.Base(policy.RecordPath(state, root)), "{")
	_, err = policy.Locate(t.Context(), state, sub, fakeDiscover(root))
	if !errors.Is(err, model.ErrEnrollment) {
		t.Errorf("a damaged record: %v", err)
	}
}

package policy_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/source"
)

// discover adapts the source package's discovery, as cmd does.
func discover(ctx context.Context, dir string) (policy.Discovery, error) {
	repo, err := source.Discover(ctx, dir)
	if err != nil {
		return policy.Discovery{}, fmt.Errorf("discover: %w", err)
	}
	return policy.Discovery{Root: repo.Root, Head: repo.Head}, nil
}

// dirs builds the per-OS directories under a temporary directory.
func dirs(t *testing.T) policy.Dirs {
	t.Helper()
	base := t.TempDir()
	return policy.Dirs{State: filepath.Join(base, "state"), Config: filepath.Join(base, "config")}
}

// input is the Load input of a directory with the example schemas.
func input(dir string, d policy.Dirs) policy.Input {
	return policy.Input{Dir: dir, Dirs: d, Version: "0.1.0", Now: now(), Schemas: schemas(), Discover: discover}
}

// enrolled commits a policy and writes the record init --apply would,
// with the example bundle installed when the policy names it.
func enrolled(t *testing.T, text string) (string, policy.Dirs) {
	t.Helper()
	root := initRepo(t)
	d := dirs(t)
	write(t, root, "golaw.yml", text)
	commitAll(t, root, "enroll")
	f := decode(t, text)
	rec := policy.Record{Root: root, PolicyDigest: policy.Digest(f), KitVersion: "0.1.0", Bootstrap: text, WrittenBy: policy.WrittenByInit, WrittenAt: now()}
	if f.Profile != nil {
		rec.ProfileDigest = f.Profile.Digest
		write(t, d.Config, "profiles/acme/profile.yml", acmeStamp)
	}
	writeRecord(t, d.State, rec)
	return root, d
}

package policy_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

func TestReadHeadAndIndex(t *testing.T) {
	t.Parallel()
	root := initRepo(t)
	write(t, root, "golaw.yml", minimalPolicy)
	commitAll(t, root, "one")
	data, found, err := policy.ReadHead(t.Context(), root)
	if err != nil || !found || string(data) != minimalPolicy {
		t.Errorf("ReadHead = %q, %v, %v", data, found, err)
	}
	_, found, err = policy.ReadIndex(t.Context(), root)
	if err != nil || !found {
		t.Errorf("the index holds the committed file too: %v, %v", found, err)
	}
	staged := minimalPolicy + "language: pt-BR\n"
	write(t, root, "golaw.yml", staged)
	gitRun(t, root, "add", "golaw.yml")
	write(t, root, "golaw.yml", "golaw: [\n")
	data, found, err = policy.ReadIndex(t.Context(), root)
	if err != nil || !found || string(data) != staged {
		t.Errorf("ReadIndex reads the index, never the working tree: %q, %v, %v", data, found, err)
	}
	if data, found, err := policy.ReadHead(t.Context(), root); err != nil || !found || string(data) != minimalPolicy {
		t.Errorf("ReadHead reads HEAD, never the index: %q, %v, %v", data, found, err)
	}
}

func TestReadAbsentAndFailing(t *testing.T) {
	t.Parallel()
	root := initRepo(t)
	write(t, root, "other.txt", "x\n")
	commitAll(t, root, "one")
	if data, found, err := policy.ReadHead(t.Context(), root); err != nil || found || data != nil {
		t.Errorf("absent at HEAD = %q, %v, %v", data, found, err)
	}
	if _, found, err := policy.ReadIndex(t.Context(), root); err != nil || found {
		t.Errorf("absent in the index = %v, %v", found, err)
	}
	unborn := initRepo(t)
	if _, _, err := policy.ReadHead(t.Context(), unborn); !errors.Is(err, model.ErrPolicy) {
		t.Errorf("an unborn HEAD cannot be listed: %v", err)
	}
	if _, _, err := policy.ReadIndex(t.Context(), t.TempDir()); !errors.Is(err, model.ErrPolicy) {
		t.Errorf("outside git: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(root, ".git", "objects")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := policy.ReadHead(t.Context(), root); !errors.Is(err, model.ErrPolicy) {
		t.Errorf("a broken object store: %v", err)
	}
}

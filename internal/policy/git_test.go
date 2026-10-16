package policy_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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

// conflicted leaves golaw.yml unresolved in the index the way a merge
// does: read-tree folds the base, ours and theirs into stages 1 to 3, or
// 2 and 3 when the base has no policy and both sides added one.
func conflicted(t *testing.T, base string) string {
	t.Helper()
	root := initRepo(t)
	write(t, root, "a.txt", "a\n")
	if base != "" {
		write(t, root, "golaw.yml", base)
	}
	commitAll(t, root, "base")
	gitRun(t, root, "branch", "base")
	gitRun(t, root, "checkout", "-q", "-b", "other")
	write(t, root, "golaw.yml", minimalPolicy+"language: en-US\n")
	commitAll(t, root, "theirs")
	gitRun(t, root, "checkout", "-q", "main")
	write(t, root, "golaw.yml", minimalPolicy+"language: pt-BR\n")
	commitAll(t, root, "ours")
	gitRun(t, root, "read-tree", "-m", "base", "main", "other")
	return root
}

func TestReadIndexRefusesAnUnresolvedMerge(t *testing.T) {
	t.Parallel()
	for name, base := range map[string]string{"modify/modify": minimalPolicy, "add/add": ""} {
		root := conflicted(t, base)
		if _, found, err := policy.ReadIndex(t.Context(), root); !errors.Is(err, model.ErrPolicy) || !strings.Contains(err.Error(), "unresolved") || found {
			t.Errorf("%s: ReadIndex = %v, %v, want ErrPolicy naming the unresolved merge", name, found, err)
		}
		l, err := policy.Load(t.Context(), input(root, dirs(t)))
		if err != nil {
			t.Fatal(err)
		}
		if _, found, err := policy.Proposed(t.Context(), l, schemas()); !errors.Is(err, model.ErrPolicy) || found {
			t.Errorf("%s: Proposed = %v, %v, want ErrPolicy", name, found, err)
		}
		gitRun(t, root, "add", "golaw.yml")
		if data, found, err := policy.ReadIndex(t.Context(), root); err != nil || !found || !strings.Contains(string(data), "pt-BR") {
			t.Errorf("%s resolved: ReadIndex = %q, %v, %v, want the staged file", name, data, found, err)
		}
	}
}

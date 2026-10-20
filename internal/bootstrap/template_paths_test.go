package bootstrap_test

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/install"
)

func TestTemplatePathsRefuseSymlinkedComponents(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, destination string
		link              bool
		parent            bool
	}{
		{"ordinary", "nested/config.txt", false, false},
		{"parent link", "nested/config.txt", true, true},
		{"destination link", "config.txt", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root, outside := t.TempDir(), t.TempDir()
			if tc.link {
				linkDestination(t, root, outside, tc.parent)
			}
			data := []byte("rendered\n")
			files := []assets.Rendered{{
				Template: assets.Template{Ownership: assets.OwnedByKit},
				Path:     tc.destination, Data: data, SHA256: install.Digest(data),
			}}
			requireLinkPlan(t, root, files, tc.link)
			if _, err := os.Stat(filepath.Join(outside, "config.txt")); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("outside destination written: %v", err)
			}
		})
	}
}

// linkDestination links the destination of the template, or its parent
// directory, to a directory outside the repository.
func linkDestination(t *testing.T, root, outside string, parent bool) {
	t.Helper()
	target, link := filepath.Join(outside, "config.txt"), filepath.Join(root, "config.txt")
	if parent {
		target, link = outside, filepath.Join(root, "nested")
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}

// requireLinkPlan requires a plan through a link to refuse with
// ErrSymlink, and any other plan to apply.
func requireLinkPlan(t *testing.T, root string, files []assets.Rendered, link bool) {
	t.Helper()
	plan, err := bootstrap.PlanTemplates(root, files)
	if link {
		if !errors.Is(err, bootstrap.ErrSymlink) {
			t.Fatalf("symlink refusal = %v", err)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if applyErr := bootstrap.ApplyTemplates(root, plan, io.Discard); applyErr != nil {
		t.Fatal(applyErr)
	}
}

func TestTemplateOutputFailureLeavesEveryDestinationUnwritten(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	plan := bootstrap.TemplatePlan{
		Changes: []install.Change{
			{Path: "first.txt", After: []byte("first\n")},
			{Path: "nested/second.txt", After: []byte("second\n")},
		},
	}
	reader, writer := io.Pipe()
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	err := bootstrap.ApplyTemplates(root, plan, writer)
	if closeErr := writer.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("output failure = %v", err)
	}
	for _, change := range plan.Changes {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(change.Path))); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("written after output failure: %s: %v", change.Path, err)
		}
	}
}

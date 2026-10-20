package bootstrap_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/policy"
)

func TestTemplatePlansPreserveRecordedLocalBaselines(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	baseline := install.Digest([]byte("original\n"))
	config := fmt.Sprintf("golaw: 1\nkit: {version: 0.1.0}\nmanaged:\n"+
		"  - {path: d.txt, sha256: %s}\n  - {path: z.txt, sha256: %s}\n", baseline, baseline)
	writeBefore(t, filepath.Join(root, bootstrap.PolicyFile), config)
	writeBefore(t, filepath.Join(root, "z.txt"), "local\n")
	plan, err := bootstrap.PlanTemplates(root, []assets.Rendered{
		templateFile("z.txt", "desired\n", assets.OwnedByKit),
		templateFile("a.txt", "new\n", assets.OwnedByKit),
		templateFile("d.txt", "desired\n", assets.OwnedByKit),
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []policy.ManagedFile{
		{Path: "a.txt", SHA256: install.Digest([]byte("new\n"))},
		{Path: "d.txt", SHA256: baseline},
		{Path: "z.txt", SHA256: baseline},
	}
	desired := install.Digest([]byte("desired\n"))
	edits := []bootstrap.LocalEdit{
		{Path: "z.txt", Recorded: baseline, Current: install.Digest([]byte("local\n")), Proposed: desired},
		{Path: "d.txt", Recorded: baseline, Current: "", Proposed: desired},
	}
	if !reflect.DeepEqual(plan.Managed, want) || !reflect.DeepEqual(plan.LocalEdits, edits) {
		t.Fatalf("managed = %+v, local = %+v", plan.Managed, plan.LocalEdits)
	}
	if applyErr := bootstrap.ApplyTemplates(root, plan, io.Discard); applyErr != nil {
		t.Fatal(applyErr)
	}
	data, err := os.ReadFile(filepath.Join(root, "z.txt"))
	if err != nil || string(data) != "local\n" {
		t.Fatalf("local bytes = %q, %v", data, err)
	}
	requireTemplateAbsent(t, root, "d.txt")
}

func TestTemplateApplyRefusesChangedInputsBeforeAnyWrites(t *testing.T) {
	t.Parallel()
	for _, ownership := range []assets.Ownership{assets.OwnedByKit, assets.OwnedMerged, assets.OwnedByDeveloper} {
		t.Run(string(ownership), func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeBefore(t, filepath.Join(root, "kept.txt"), "desired\n")
			plan, err := bootstrap.PlanTemplates(root, []assets.Rendered{
				templateFile("new.txt", "new\n", assets.OwnedByKit),
				templateFile("kept.txt", "desired\n", ownership),
			})
			if err != nil {
				t.Fatal(err)
			}
			writeBefore(t, filepath.Join(root, "kept.txt"), "changed\n")
			var out bytes.Buffer
			if err := bootstrap.ApplyTemplates(root, plan, &out); !errors.Is(err, install.ErrChanged) {
				t.Fatalf("changed input = %v", err)
			}
			if out.Len() != 0 {
				t.Fatalf("invalid plan produced output: %q", out.String())
			}
			requireTemplateAbsent(t, root, "new.txt")
		})
	}
}

func TestTemplateApplyRechecksAfterEveryDiffIsPrinted(t *testing.T) {
	t.Parallel()
	for _, link := range []bool{false, true} {
		t.Run(strconv.FormatBool(link), func(t *testing.T) {
			t.Parallel()
			requireRecheckedTemplatePlan(t, link)
		})
	}
}

func requireRecheckedTemplatePlan(t *testing.T, link bool) {
	t.Helper()
	root, outside := t.TempDir(), t.TempDir()
	plan, err := bootstrap.PlanTemplates(root, []assets.Rendered{
		templateFile("first.txt", "first\n", assets.OwnedByKit),
		templateFile("nested/last.txt", "last\n", assets.OwnedByKit),
	})
	if err != nil {
		t.Fatal(err)
	}
	writer := &templateMutationWriter{mutate: func() {
		if link {
			if linkErr := os.Symlink(outside, filepath.Join(root, "nested")); linkErr != nil {
				t.Fatal(linkErr)
			}
			return
		}
		writeBefore(t, filepath.Join(root, "first.txt"), "changed\n")
	}}
	want := install.ErrChanged
	if link {
		want = bootstrap.ErrSymlink
	}
	if applyErr := bootstrap.ApplyTemplates(root, plan, writer); !errors.Is(applyErr, want) {
		t.Fatalf("changed during output = %v", applyErr)
	}
	requireTemplateAbsent(t, outside, "last.txt")
	if link {
		requireTemplateAbsent(t, root, "first.txt")
	}
}

func TestTemplatePathsRefuseInvalidAndIrregularDestinations(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"", ".", "../escape.txt", "/outside.txt", "nested/../escape.txt", "C:outside", `nested\outside`, "directory", "file/child"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if err := os.Mkdir(filepath.Join(root, "directory"), 0o700); err != nil {
				t.Fatal(err)
			}
			writeBefore(t, filepath.Join(root, "file"), "regular\n")
			_, err := bootstrap.PlanTemplates(root, []assets.Rendered{templateFile(name, "new\n", assets.OwnedByKit)})
			if !errors.Is(err, install.ErrPath) {
				t.Fatalf("invalid path = %v", err)
			}
		})
	}
}

func TestTemplateDiffKeepsGroupsRelativeAndLocalized(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ language, writes, proposals string }{
		{"", "Gravações:", "Propostas sem gravação:"},
		{"pt-BR", "Gravações:", "Propostas sem gravação:"},
		{"en", "Writes:", "Proposals without writes:"},
	} {
		t.Run(tc.language, func(t *testing.T) {
			t.Parallel()
			plan := bootstrap.TemplatePlan{
				Language: tc.language,
				Changes:  []install.Change{{Path: "a.txt", After: []byte("new\n")}},
				Proposed: []install.Change{{Path: "b.txt", Before: []byte("old\n"), After: []byte("wanted\n"), Existed: true}},
			}
			var out bytes.Buffer
			if err := bootstrap.WriteTemplateDiff(&out, plan); err != nil {
				t.Fatal(err)
			}
			want := tc.writes + "\n--- a.txt\n+++ a.txt\n+new\n" +
				tc.proposals + "\n--- b.txt\n+++ b.txt\n-old\n+wanted\n"
			if out.String() != want {
				t.Fatalf("diff = %q, want %q", out.String(), want)
			}
		})
	}
}

func TestTemplateLateOutputFailureWritesNothing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	plan, err := bootstrap.PlanTemplates(root, []assets.Rendered{templateFile("a.txt", "new\n", assets.OwnedByKit)})
	if err != nil {
		t.Fatal(err)
	}
	writer := &templateMutationWriter{fail: true}
	if err := bootstrap.ApplyTemplates(root, plan, writer); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("diff failure = %v", err)
	}
	requireTemplateAbsent(t, root, "a.txt")
}

func templateFile(path, data string, ownership assets.Ownership) assets.Rendered {
	return assets.Rendered{
		Template: assets.Template{Ownership: ownership},
		Path:     path, Data: []byte(data), SHA256: install.Digest([]byte(data)),
	}
}

func requireTemplateAbsent(t *testing.T, root, name string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(name))); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("%s unexpectedly exists: %v", name, err)
	}
}

type templateMutationWriter struct {
	mutate func()
	fail   bool
	calls  int
}

func (w *templateMutationWriter) Write(data []byte) (int, error) {
	w.calls++
	if w.fail && w.calls > 1 {
		return 0, io.ErrClosedPipe
	}
	if w.mutate != nil && strings.Contains(string(data), "Gravações:") {
		w.mutate()
		w.mutate = nil
	}
	return len(data), nil
}

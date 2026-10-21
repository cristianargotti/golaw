package bootstrap_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw"
	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/install"
)

func TestTemplatePlanRefusesInvalidInputs(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, policy, path string }{
		{"invalid policy", "unknown: true\n", "a.txt"},
		{"policy destination", "", bootstrap.PolicyFile},
		{"unknown ownership", "", "a.txt"},
		{"duplicate destination", "", "a.txt"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeBefore(t, filepath.Join(root, bootstrap.PolicyFile), tc.policy)
			files := []assets.Rendered{templateFile(tc.path, "new\n", assets.OwnedByKit)}
			if tc.name == "unknown ownership" {
				files[0].Template.Ownership = "unknown"
			}
			if tc.name == "duplicate destination" {
				files = append(files, files[0])
			}
			if _, err := bootstrap.PlanTemplates(root, files); err == nil {
				t.Fatal("invalid input accepted")
			}
		})
	}
}

func TestTemplatePlanKeepsEmptyExistingDeveloperFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "empty.txt"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	plan, err := bootstrap.PlanTemplates(root, []assets.Rendered{
		templateFile("empty.txt", "new\n", assets.OwnedByDeveloper),
	})
	if err != nil || len(plan.Changes) != 0 || len(plan.Proposed) != 0 || len(plan.Managed) != 0 {
		t.Fatalf("existing empty file plan = %+v, %v", plan, err)
	}
}

func TestTemplateMergedProposalAppendsEachMissingOccurrence(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeBefore(t, filepath.Join(root, "Makefile"), "all:\n\tbuild")
	plan, err := bootstrap.PlanTemplates(root, []assets.Rendered{
		templateFile("Makefile", "all:\ninclude golaw.mk\ninclude golaw.mk\n", assets.OwnedMerged),
	})
	if err != nil || len(plan.Proposed) != 1 {
		t.Fatalf("merge plan = %+v, %v", plan, err)
	}
	if got := string(plan.Proposed[0].After); got != "all:\n\tbuild\ninclude golaw.mk\ninclude golaw.mk\n" {
		t.Fatalf("merged bytes = %q", got)
	}
	empty, err := bootstrap.PlanTemplates(root, []assets.Rendered{
		templateFile("Makefile", "", assets.OwnedMerged),
	})
	if err != nil || len(empty.Proposed) != 0 {
		t.Fatalf("empty merge = %+v, %v", empty, err)
	}
}

func TestTemplateMergedEditorConfigPreservesRequiredOccurrences(t *testing.T) {
	t.Parallel()
	file, err := assets.Render(golaw.FS, assets.Template{
		Source: "templates/.editorconfig.tmpl", Destination: ".editorconfig", Ownership: assets.OwnedMerged,
	}, assets.Values{})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.FieldsFunc(string(file.Data), func(r rune) bool { return r == '\n' })
	compact := strings.Join(lines, "\n") + "\n"
	slices.Reverse(lines)
	shuffled := strings.Join(lines, "\n") + "\n"
	const brownfield = "root = true\n\n[*]\nindent_style = space\n"
	const appended = "charset = utf-8\nend_of_line = lf\ninsert_final_newline = true\n" +
		"trim_trailing_whitespace = true\nindent_size = 2\n\n[*.go]\nindent_style = tab\n" +
		"\n[Makefile]\nindent_style = tab\n\n[*.mk]\nindent_style = tab\n"
	for _, tc := range []struct{ name, before, want string }{
		{"brownfield", brownfield, brownfield + appended},
		{"without blank lines", compact, compact},
		{"reordered lines under other headers", shuffled, "indent_style = tab\nroot = true\n[*.mk]\nindent_style = tab\n[Makefile]\nindent_style = tab\n[*.go]\nindent_size = 2\nindent_style = space\ntrim_trailing_whitespace = true\ninsert_final_newline = true\nend_of_line = lf\ncharset = utf-8\nindent_style = tab\n[*]\nroot = true\ncharset = utf-8\nend_of_line = lf\ninsert_final_newline = true\ntrim_trailing_whitespace = true\nindent_style = space\nindent_size = 2\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			path := filepath.Join(root, file.Path)
			writeBefore(t, path, tc.before)
			plan, planErr := bootstrap.PlanTemplates(root, []assets.Rendered{file})
			if planErr != nil || len(plan.Changes) != 0 || len(plan.Managed) != 0 {
				t.Fatalf("merged plan = %+v, %v", plan, planErr)
			}
			requireMergedTemplateProposal(t, plan, tc.before, tc.want)
			if current, readErr := os.ReadFile(path); readErr != nil || !bytes.Equal(current, []byte(tc.before)) {
				t.Fatalf("merged file changed: %q, %v", current, readErr)
			}
		})
	}
}

func requireMergedTemplateProposal(t *testing.T, plan bootstrap.TemplatePlan, before, want string) {
	t.Helper()
	if before == want {
		if len(plan.Proposed) != 0 {
			t.Fatalf("complete merged file proposed again: %d proposals", len(plan.Proposed))
		}
		return
	}
	if len(plan.Proposed) != 1 {
		t.Fatalf("merged proposals = %d, want 1", len(plan.Proposed))
	}
	if got := string(plan.Proposed[0].After); got != want {
		t.Fatalf("merged bytes = %q, want %q", got, want)
	}
}

func TestTemplateMergedProposalsPreserveCRLF(t *testing.T) {
	t.Parallel()
	editorConfig, err := assets.Render(golaw.FS, assets.Template{
		Source: "templates/.editorconfig.tmpl", Destination: ".editorconfig", Ownership: assets.OwnedMerged,
	}, assets.Values{})
	if err != nil {
		t.Fatal(err)
	}
	makefile := templateFile("Makefile", "include golaw.mk\n", assets.OwnedMerged)
	const makeBefore = "all:\r\n\tgo build ./...\r\n"
	const makeAfter = makeBefore + "include golaw.mk\r\n"
	const editorBefore = "root = true\r\n\r\n[*]\r\nindent_style = space\r\n"
	const editorAfter = editorBefore + "charset = utf-8\r\nend_of_line = lf\r\ninsert_final_newline = true\r\n" +
		"trim_trailing_whitespace = true\r\nindent_size = 2\r\n\r\n[*.go]\r\nindent_style = tab\r\n" +
		"\r\n[Makefile]\r\nindent_style = tab\r\n\r\n[*.mk]\r\nindent_style = tab\r\n"
	for _, tc := range []struct {
		name, before, want string
		file               assets.Rendered
	}{
		{"already included", makeAfter, makeAfter, makefile},
		{"missing include", makeBefore, makeAfter, makefile},
		{"editorconfig", editorBefore, editorAfter, editorConfig},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			path := filepath.Join(root, tc.file.Path)
			writeBefore(t, path, tc.before)
			plan, planErr := bootstrap.PlanTemplates(root, []assets.Rendered{tc.file})
			if planErr != nil || len(plan.Changes) != 0 || len(plan.Managed) != 0 {
				t.Fatalf("merged plan = %+v, %v", plan, planErr)
			}
			requireMergedTemplateProposal(t, plan, tc.before, tc.want)
			if current, readErr := os.ReadFile(path); readErr != nil || !bytes.Equal(current, []byte(tc.before)) {
				t.Fatalf("merged file changed: %q, %v", current, readErr)
			}
		})
	}
}

func TestTemplateApplyRefusesPolicyChangedWhilePrinting(t *testing.T) {
	t.Parallel()
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "absent", true: "existing"}[existing], func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			const config = "golaw: 1\nkit: {version: 0.1.0}\n"
			if existing {
				writeBefore(t, filepath.Join(root, bootstrap.PolicyFile), config)
			}
			plan, err := bootstrap.PlanTemplates(root, []assets.Rendered{templateFile("a.txt", "new\n", assets.OwnedByKit)})
			if err != nil {
				t.Fatal(err)
			}
			writer := &templateMutationWriter{mutate: func() {
				writeBefore(t, filepath.Join(root, bootstrap.PolicyFile), config+"language: en\n")
			}}
			if applyErr := bootstrap.ApplyTemplates(root, plan, writer); !errors.Is(applyErr, install.ErrChanged) {
				t.Fatalf("changed policy = %v", applyErr)
			}
			requireTemplateAbsent(t, root, "a.txt")
		})
	}
}

func TestTemplateRootsResolveAliasesAndRejectReplacement(t *testing.T) {
	t.Parallel()
	root, outside := t.TempDir(), t.TempDir()
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	plan, err := bootstrap.PlanTemplates(alias, []assets.Rendered{templateFile("a.txt", "new\n", assets.OwnedByKit)})
	if err != nil {
		t.Fatal(err)
	}
	if applyErr := bootstrap.ApplyTemplates(outside, plan, io.Discard); !errors.Is(applyErr, install.ErrChanged) {
		t.Fatalf("replaced root = %v", applyErr)
	}
	var out bytes.Buffer
	if applyErr := bootstrap.ApplyTemplates(alias, plan, &out); applyErr != nil {
		t.Fatal(applyErr)
	}
	if strings.Contains(out.String(), root) || strings.Contains(out.String(), alias) {
		t.Fatalf("absolute root leaked into diff: %q", out.String())
	}
}

func TestTemplatePlanDropsRecordsOfKnownNonKitFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	baseline := install.Digest([]byte("baseline\n"))
	config := fmt.Sprintf("golaw: 1\nkit: {version: 0.1.0}\nmanaged:\n"+
		"  - {path: Makefile, sha256: %s}\n"+
		"  - {path: notes.md, sha256: %s}\n"+
		"  - {path: kit.txt, sha256: %s}\n", baseline, baseline, baseline)
	writeBefore(t, filepath.Join(root, bootstrap.PolicyFile), config)
	for _, name := range []string{"Makefile", "notes.md", "kit.txt"} {
		writeBefore(t, filepath.Join(root, name), "local\n")
	}
	plan, err := bootstrap.PlanTemplates(root, []assets.Rendered{
		templateFile("Makefile", "include golaw.mk\n", assets.OwnedMerged),
		templateFile("notes.md", "documentation\n", assets.OwnedByDeveloper),
		templateFile("kit.txt", "desired\n", assets.OwnedByKit),
	})
	if err != nil || len(plan.Managed) != 1 || plan.Managed[0].Path != "kit.txt" || plan.Managed[0].SHA256 != baseline {
		t.Fatalf("ownership records = %+v, %v", plan.Managed, err)
	}
}

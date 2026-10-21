package bootstrap_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw"
	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/bootstrap"
)

// birthEditorConfig is the .editorconfig a scaffolded service is born
// with: indent_style = tab twice, under [*.go] and under a module section,
// and no [Makefile] section, so a merge that counts lines finds both lines
// present and leaves recipes to the spaces of [*].
const birthEditorConfig = "# top-most EditorConfig file\nroot = true\n\n[*]\nindent_style = space\nindent_size = 2\n" +
	"end_of_line = lf\ncharset = utf-8\ntrim_trailing_whitespace = true\ninsert_final_newline = true\n\n" +
	"[*.go]\nindent_style = tab\nindent_size = 4\n[{go.mod,go.sum}]\nindent_style = tab\nindent_size = 4\n\n" +
	"[*.md]\ntrim_trailing_whitespace = false\n"

// starSection is the [*] section of the asset, every property present.
const starSection = "[*]\ncharset = utf-8\nend_of_line = lf\ninsert_final_newline = true\n" +
	"trim_trailing_whitespace = true\nindent_style = space\nindent_size = 2\n"

func renderedEditorConfig(t *testing.T) assets.Rendered {
	t.Helper()
	file, err := assets.Render(golaw.FS, assets.Template{
		Source: "templates/.editorconfig.tmpl", Destination: ".editorconfig", Ownership: assets.OwnedMerged,
	}, assets.Values{})
	if err != nil {
		t.Fatal(err)
	}
	return file
}

func TestTemplateMergedEditorConfigMergesBySection(t *testing.T) {
	t.Parallel()
	file := renderedEditorConfig(t)
	const goSection = "\n[*.go]\nindent_style = tab\n"
	const kitSections = "\n[Makefile]\nindent_style = tab\n\n[*.mk]\nindent_style = tab\n"
	for _, tc := range []struct{ name, before, want string }{
		{"red birth without a Makefile section", birthEditorConfig, birthEditorConfig + kitSections},
		{
			"red Makefile section with spaces",
			"root = true\n\n" + starSection + goSection + "\n[Makefile]\nindent_style = space\n\n[*.mk]\nindent_style = tab\n",
			"root = true\n\n" + starSection + goSection + "\n[Makefile]\nindent_style = space\nindent_style = tab\n\n[*.mk]\nindent_style = tab\n",
		},
		{"red file ending in a blank line", "root = true\n\n" + starSection + goSection + "\n", "root = true\n\n" + starSection + goSection + kitSections},
		{
			"red without a preamble", "[*]\nindent_style = space\n",
			"root = true\n[*]\nindent_style = space\ncharset = utf-8\nend_of_line = lf\ninsert_final_newline = true\n" +
				"trim_trailing_whitespace = true\nindent_size = 2\n" + goSection + kitSections,
		},
		{"near sections in another order", "root = true\n" + goSection + "\n" + starSection + "\n[*.mk]\nindent_style = tab\n\n[Makefile]\nindent_style = tab\n", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeBefore(t, filepath.Join(root, file.Path), tc.before)
			plan, err := bootstrap.PlanTemplates(root, []assets.Rendered{file})
			if err != nil {
				t.Fatal(err)
			}
			want := tc.want
			if want == "" {
				want = tc.before
			}
			requireMergedTemplateProposal(t, plan, tc.before, want)
		})
	}
}

func TestTemplateMergedGitattributesStaysLineBased(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const before = "[attr]binary -diff\n*.png binary\n"
	writeBefore(t, filepath.Join(root, ".gitattributes"), before)
	plan, err := bootstrap.PlanTemplates(root, []assets.Rendered{
		templateFile(".gitattributes", "* text=auto eol=lf\n*.png binary\n", assets.OwnedMerged),
	})
	if err != nil {
		t.Fatal(err)
	}
	requireMergedTemplateProposal(t, plan, before, before+"* text=auto eol=lf\n")
}

func TestTemplateMergedEditorConfigPreservesSpacing(t *testing.T) {
	t.Parallel()
	file := renderedEditorConfig(t)
	complete := string(file.Data)
	const before = "root = true\n\n" + starSection + "\n[*.go]\nindent_style = tab\n \t\n"
	const after = before + "[Makefile]\nindent_style = tab\n\n[*.mk]\nindent_style = tab\n"
	for _, tc := range []struct{ name, before, want string }{
		{"no final newline", strings.TrimSuffix(complete, "\n"), ""},
		{"padded lines", " \t" + strings.ReplaceAll(complete, "\n", " \t\n \t"), ""},
		{"whitespace separator", before, after},
		{"CRLF whitespace separator", strings.ReplaceAll(before, "\n", "\r\n"), strings.ReplaceAll(after, "\n", "\r\n")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeBefore(t, filepath.Join(root, file.Path), tc.before)
			plan, err := bootstrap.PlanTemplates(root, []assets.Rendered{file})
			if err != nil {
				t.Fatal(err)
			}
			want := tc.want
			if want == "" {
				want = tc.before
			}
			requireMergedTemplateProposal(t, plan, tc.before, want)
		})
	}
}

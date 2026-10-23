package rung_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/rung"
)

// dash is the em dash, built from its code point so this file carries none.
var dash = string(rune(0x2014))

func TestCommitMsgJudgesTheMessageGitWillRecord(t *testing.T) {
	t.Parallel()
	r := armedRepo(t, armedPolicy)
	cases := map[string]struct {
		text string
		code int
		want string
	}{
		"a conventional subject": {"feat: add the reader\n", 0, "verdict clean: 0 findings"},
		"a bad subject":          {"Update the reader\n", 1, "-:1:1 deny commit.conventional the subject \"Update the reader\""},
		"comments and a diff":    {"fix: keep it\n\n# a comment " + dash + "\n#" + " ------------------------ >8 ------------------------\n+a " + dash + "\n", 0, "verdict clean"},
		"a trailer":              {"fix: keep it\n\nReviewed-by: someone\n", 1, "commit.no-trailers line 3 is a commit trailer (Reviewed-by)"},
		"a dash in the body":     {"fix: keep it\n\nfirst " + dash + " second\n", 1, "-:3:7 deny commit.no-dashes"},
		"a long body line":       {"fix: keep it\n\n" + strings.Repeat("a", 101) + "\n", 1, "commit.body-width"},
	}
	for name, c := range cases {
		out := rung.RunCommitMsg(t.Context(), deps(t, r), message(r, c.text))
		if out.Code != c.code || !contains(out, c.want) {
			t.Errorf("%s: %+v, want code %d and %q", name, out, c.code, c.want)
		}
	}
}

func TestCommitMsgSpeaksToThePersonOnlyWhenItRefuses(t *testing.T) {
	t.Parallel()
	r := armedRepo(t, armedPolicy)
	refused := rung.RunCommitMsg(t.Context(), deps(t, r), message(r, "Update the reader\n"))
	last := refused.Lines[len(refused.Lines)-1]
	if refused.Code != 1 || last != "golaw bloqueou a operação: assunto do commit fora do padrão convencional; escreva tipo(escopo): descrição, com a descrição começando em letra minúscula (verificação commit.conventional)." {
		t.Errorf("refusal = %+v", refused)
	}
	english := armedRepo(t, armedPolicy+"language: en\n")
	if out := rung.RunCommitMsg(t.Context(), deps(t, english), message(english, "Update the reader\n")); !strings.HasPrefix(out.Lines[len(out.Lines)-1], "golaw blocked the operation: commit.conventional: the subject") {
		t.Errorf("English refusal = %+v", out)
	}
	clean := rung.RunCommitMsg(t.Context(), deps(t, r), message(r, "feat: add the reader\n"))
	if clean.Code != 0 || len(clean.Lines) != 1 || !strings.HasPrefix(clean.Lines[0], "verdict clean: 0 findings (0 enforced, 0 notes), ") {
		t.Errorf("clean = %+v", clean)
	}
}

func TestCommitMsgResolvesTheCleanupGitApplies(t *testing.T) {
	t.Parallel()
	comment := "fix: keep it\n\n# note " + dash + "\n"
	r := armedRepo(t, armedPolicy)
	if out := rung.RunCommitMsg(t.Context(), deps(t, r), message(r, comment)); out.Code != 0 {
		t.Errorf("an edited message loses its comments: %+v", out)
	}
	if out := rung.RunCommitMsg(t.Context(), deps(t, r, "GIT_EDITOR=:"), message(r, comment)); out.Code != 1 || !contains(out, "commit.no-dashes") {
		t.Errorf("a message git does not edit keeps its comments: %+v", out)
	}
	r.Git("config", "commit.cleanup", "verbatim")
	if out := rung.RunCommitMsg(t.Context(), deps(t, r), message(r, comment)); out.Code != 1 {
		t.Errorf("commit.cleanup verbatim keeps the comment: %+v", out)
	}
	r.Git("config", "commit.cleanup", "strip")
	r.Git("config", "core.commentChar", ";")
	semicolon := "fix: keep it\n\n; note " + dash + "\n"
	if out := rung.RunCommitMsg(t.Context(), deps(t, r), message(r, semicolon)); out.Code != 0 {
		t.Errorf("a configured comment character: %+v", out)
	}
	if out := rung.RunCommitMsg(t.Context(), deps(t, r), message(r, comment)); out.Code != 1 {
		t.Errorf("a line of another comment character is recorded: %+v", out)
	}
	r.Git("config", "commit.cleanup", "sometimes")
	if out := rung.RunCommitMsg(t.Context(), deps(t, r), message(r, comment)); out.Code != 2 || !contains(out, "sometimes") {
		t.Errorf("a cleanup git does not know: %+v", out)
	}
}

func TestCommitMsgMeasuresTheIndexGitCommits(t *testing.T) {
	t.Parallel()
	r := armedRepo(t, armedPolicy)
	r.Stage(files(41))
	if out := rung.RunCommitMsg(t.Context(), deps(t, r), message(r, "chore: dump the tables\n")); out.Code != 1 || !contains(out, "a chore commit adds 41 new files") {
		t.Errorf("41 staged files: %+v", out)
	}
	if out := rung.RunCommitMsg(t.Context(), deps(t, r), message(r, "feat: add the tables\n")); out.Code != 0 {
		t.Errorf("a feat commit: %+v", out)
	}
	other := armedRepo(t, armedPolicy)
	index := other.AlternateIndex()
	for path, content := range files(41) {
		other.Write(path, content)
	}
	other.GitEnv([]string{"GIT_INDEX_FILE=" + index}, "add", "tables")
	if out := rung.RunCommitMsg(t.Context(), deps(t, other), message(other, "chore: dump the tables\n")); out.Code != 0 {
		t.Errorf("the index of the repository holds nothing staged: %+v", out)
	}
	alternate := deps(t, other, "GIT_INDEX_FILE="+index)
	if out := rung.RunCommitMsg(t.Context(), alternate, message(other, "chore: dump the tables\n")); out.Code != 1 {
		t.Errorf("the index git commits through: %+v", out)
	}
	huge := armedRepo(t, armedPolicy)
	huge.Stage(map[string]string{"dump.txt": strings.Repeat("a\n", 4<<20+1)})
	out := rung.RunCommitMsg(t.Context(), deps(t, huge), message(huge, "chore: dump the table\n"))
	if out.Code != 0 || out.Lines[len(out.Lines)-1] != "golaw commit-msg: over the read bound, not judged here: dump.txt; the pipeline judges them" {
		t.Errorf("a staged file over the read bound is left to the pipeline: %+v", out)
	}
}

func TestCommitMsgLeavesOutWhatAMergeBrought(t *testing.T) {
	t.Parallel()
	r := armedRepo(t, armedPolicy)
	r.Branch("side", files(41))
	r.Commit("docs: say why", map[string]string{"notes.md": "why\n"})
	r.MergeInProgress("side")
	if out := rung.RunCommitMsg(t.Context(), deps(t, r), message(r, "chore: take the tables of the side branch\n")); out.Code != 0 {
		t.Errorf("what the merge brought is the pipeline's: %+v", out)
	}
}

func TestCommitMsgRefusesAMessageItCannotRead(t *testing.T) {
	t.Parallel()
	r := armedRepo(t, armedPolicy)
	missing := rung.RunCommitMsg(t.Context(), deps(t, r), filepath.Join(r.Root, "absent", "COMMIT_EDITMSG"))
	if missing.Code != 2 || !strings.HasPrefix(missing.Lines[0], "golaw commit-msg: read the commit message: ") || len(missing.Lines) != 2 {
		t.Errorf("missing file: %+v", missing)
	}
	huge := rung.RunCommitMsg(t.Context(), deps(t, r), message(r, "fix: x\n\n"+strings.Repeat("a\n", 4<<20+1)))
	if huge.Code != 2 || !contains(huge, "larger than the read bound") {
		t.Errorf("a message over the bound: %+v", huge.Code)
	}
	off := armedRepo(t, armedPolicy+"ladder: {commit_msg: false}\n")
	if out := rung.RunCommitMsg(t.Context(), deps(t, off), "absent"); out.Code != 0 || len(out.Lines) != 0 {
		t.Errorf("a rung switched off reads no message: %+v", out)
	}
}

// Under core.commentChar=auto git picks the first of #;@!$%^&|: that starts
// no line of the message it prepares: a prepared message that holds # lines
// gets ;, keeps the # lines and drops the verbose diff below its ; scissors
// line. The message comes from a file, because a conflicted merge no longer
// holds # lines once git writes its Conflicts section as a comment. The rung
// reads the choice from the message git wrote, captured here by an editor
// that copies it, so the commit goes through.
func TestCommitMsgReadsTheCommentCharacterGitPicked(t *testing.T) {
	t.Parallel()
	r := armedRepo(t, armedPolicy)
	r.Commit("chore: add the table", map[string]string{"a.txt": "base\n"})
	r.Stage(map[string]string{"a.txt": "side " + strings.Repeat("x", 120) + "\n"})
	r.Git("config", "core.commentChar", "auto")
	r.Git("config", "commit.verbose", "true")
	prepared := filepath.Join(t.TempDir(), "prepared")
	if err := os.WriteFile(prepared, []byte("feat: change the table\n\n# Conflicts:\n#\ta.txt\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	captured := filepath.ToSlash(filepath.Join(t.TempDir(), "COMMIT_EDITMSG"))
	r.GitEnv([]string{"GIT_EDITOR=cp \"$1\" '" + captured + "'; true"}, "commit", "-q", "-e", "-F", filepath.ToSlash(prepared))
	if recorded := r.Git("log", "-1", "--format=%B"); !strings.Contains(recorded, "# Conflicts:") {
		t.Fatalf("git dropped the # lines, so it did not pick ; under auto: %q", recorded)
	}
	if out := rung.RunCommitMsg(t.Context(), deps(t, r), captured); out.Code != 0 {
		t.Errorf("a verbose merge under core.commentChar=auto = %+v", out)
	}
	plain := armedRepo(t, armedPolicy)
	plain.Git("config", "core.commentChar", "auto")
	if out := rung.RunCommitMsg(t.Context(), deps(t, plain), message(plain, "fix: keep it\n\nthe body\n# a comment git drops "+dash+"\n")); out.Code != 0 {
		t.Errorf("a # comment git drops under auto = %+v", out)
	}
	if out := rung.RunCommitMsg(t.Context(), deps(t, plain), message(plain, "fix: keep it\n\nthe body "+dash+"\n# a comment\n")); out.Code != 1 || !contains(out, "commit.no-dashes") {
		t.Errorf("a dash in the body under auto = %+v", out)
	}
}

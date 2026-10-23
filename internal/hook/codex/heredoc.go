package codex

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// heredoc is a command the recognizer reads as a write (D9): a patch
// Codex applies itself, from the working directory or after a cd of one
// word, or a cat whose here-document the shell writes to one visible file,
// replaced or appended to; body is the here-document as the file or the
// patch receives it.
type heredoc struct {
	patch   bool
	moved   bool
	dir     string
	target  string
	appends bool
	body    string
}

// The command names the recognizer reads; Codex intercepts both spellings
// of the patch tool (survey section 2.4).
const (
	patchName = "apply_patch"
	patchHost = "applypatch"
	cdName    = "cd"
	catName   = "cat"
)

// recognize reads a command whose only statement is one of the forms of
// D9. Anything else, a second statement, another connector, a pipe or a
// word whose value the shell would expand, is not one, and stays the shell
// family's with the detection of its post event.
func recognize(command string) (heredoc, bool) {
	first, rest, found := strings.Cut(strings.TrimLeft(command, " \t\n"), "\n")
	if !found {
		return heredoc{}, false
	}
	tokens, ok := lexLine(first)
	if !ok {
		return heredoc{}, false
	}
	form, delimiter, ok := formOf(tokens)
	if !ok || delimiter.word == "" {
		return heredoc{}, false
	}
	body, ok := heredocBody(rest, delimiter.word)
	// The shell expands an unquoted here-document and removes its escaped
	// line breaks, so its raw text is not what reaches the file; Codex reads
	// a patch raw whatever the quoting.
	if !ok || !form.patch && !delimiter.quoted && strings.ContainsAny(body, "$`\\") {
		return heredoc{}, false
	}
	form.body = body
	return form, true
}

// formOf matches the tokens of the opening line against the forms of D9
// and answers the form with its delimiter word.
func formOf(tokens []token) (heredoc, token, bool) {
	switch {
	case len(tokens) == 3 && patchCommand(tokens[0]) && tokens[1].op == heredocOp && isWord(tokens[2]):
		return heredoc{patch: true}, tokens[2], true
	case len(tokens) == 6 && named(tokens[0], cdName) && isWord(tokens[1]) && tokens[2].op == andOp &&
		patchCommand(tokens[3]) && tokens[4].op == heredocOp && isWord(tokens[5]):
		return heredoc{patch: true, moved: true, dir: tokens[1].word}, tokens[5], true
	case len(tokens) == 5 && named(tokens[0], catName):
		return catForm(tokens[1:])
	default:
		return heredoc{}, token{}, false
	}
}

// catForm reads the two redirections of a cat, in either order: one
// visible target written or appended to, and the here-document. A target
// that starts with ~ or carries surrounding white space is left to the
// shell, which would name another file than its literal value.
func catForm(tokens []token) (heredoc, token, bool) {
	redirect, delimiter := tokens[:2], tokens[2:]
	if tokens[0].op == heredocOp {
		redirect, delimiter = tokens[2:], tokens[:2]
	}
	target := redirect[1]
	if delimiter[0].op != heredocOp || !isWord(delimiter[1]) || !isWord(target) ||
		redirect[0].op != writeOp && redirect[0].op != appendOp {
		return heredoc{}, token{}, false
	}
	if target.word == "" || strings.HasPrefix(target.word, "~") || strings.TrimSpace(target.word) != target.word {
		return heredoc{}, token{}, false
	}
	return heredoc{target: target.word, appends: redirect[0].op == appendOp}, delimiter[1], true
}

func isWord(t token) bool { return t.op == "" }

// named tells an unquoted command name, as the parse of Codex reads one.
func named(t token, name string) bool { return isWord(t) && !t.quoted && t.word == name }

func patchCommand(t token) bool { return named(t, patchName) || named(t, patchHost) }

// heredocBody is the text after the opening line up to the first line that
// is the delimiter, each line with its line feed; only white space may
// follow that line, or the command holds another statement.
func heredocBody(rest, delimiter string) (string, bool) {
	for at := 0; ; {
		line, next, found := strings.Cut(rest[at:], "\n")
		if line == delimiter {
			return rest[:at], strings.TrimSpace(next) == ""
		}
		if !found {
			return "", false
		}
		at += len(line) + 1
	}
}

// patchDir is the directory Codex applies a recognized patch in: the
// working directory, or the cd word joined to it literally, an absolute
// word replacing it, so a word that starts with ~ names a directory under
// the working directory, never the home (survey section 2.4).
func patchDir(cwd string, form heredoc) string {
	if !form.moved {
		return cwd
	}
	return joined(cwd, form.dir)
}

// joined is a path joined to a directory as Codex joins it, cleaned and in
// the host's separators, an absolute path kept.
func joined(dir, path string) string {
	if native := filepath.FromSlash(path); filepath.IsAbs(native) {
		return filepath.Clean(native)
	}
	return filepath.Join(filepath.FromSlash(dir), filepath.FromSlash(path))
}

// heredocTargets are the files a recognized patch writes, for enrollment
// before any file is read: the first path it writes and then the others,
// each where Codex writes it, so enrollment answers for that location
// and a patch outside every repository keeps the valid allow (A3).
func heredocTargets(cwd, command string) (string, []string) {
	form, ok := recognize(command)
	if !ok || !form.patch {
		return "", nil
	}
	hunks, err := ParsePatch(form.body)
	if err != nil || len(hunks) == 0 {
		return "", nil
	}
	dir := patchDir(cwd, form)
	var paths []string
	for _, hunk := range hunks {
		for _, path := range []string{hunk.Path, hunk.To} {
			if path == "" {
				continue
			}
			if name := joined(dir, path); !slices.Contains(paths, name) {
				paths = append(paths, name)
			}
		}
	}
	return paths[0], paths[1:]
}

// heredocPatch is the request of a patch Codex applies itself, which runs
// no other event (survey section 2.4): the shell event with the candidates
// of the patch, reconstructed where Codex writes them, as its writes,
// marked intercepted, since no post event reads a baseline of it. An empty
// patch writes nothing, and one whose base is over the read bound is
// answered as the patch tool's own event, judged by its names.
func heredocPatch(request hook.Request, form heredoc, base hook.Base, snapshot model.Snapshot) (hook.Request, error) {
	event := request.Event
	event.Shell, event.Cwd = nil, patchDir(event.Cwd, form)
	written, err := patchRequest(event, form.body, base, snapshot)
	switch {
	case err != nil:
		return hook.Request{}, err
	case written.Observe:
		return request, nil
	case len(written.Gaps) > 0:
		return written, nil
	}
	request.Writes = &hook.Writes{Candidates: written.Event.Candidates, Content: written.Content, Intercepted: true}
	return request, nil
}

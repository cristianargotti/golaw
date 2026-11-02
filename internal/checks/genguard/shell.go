package genguard

import (
	"path/filepath"
	"strings"

	"github.com/cristianargotti/golaw/internal/checks/shell"
	"github.com/cristianargotti/golaw/internal/model"
)

// agentEvent reports whether the input is an agent's write or shell
// command before it runs: a shell command writes a file as surely as a
// write tool when its words name the file literally (M7.9.2).
func agentEvent(in model.Input) bool {
	return agentWrite(in) || in.Mode == model.ModeHook && in.Event != nil && in.Event.Shape == model.ShapeShell
}

// hookPaths are the repository paths the event writes or leaves: every
// candidate and the path a move leaves, or the literal targets of a shell
// command.
func hookPaths(in model.Input) []string {
	if in.Event.Shape == model.ShapeShell {
		return shellTargets(in)
	}
	return judgedPaths(in.Event.Candidates)
}

// shellTargets answers the repository paths of the literal targets of a
// shell command; a payload without a command or a root names none, since a
// relative target cannot be placed without them.
func shellTargets(in model.Input) []string {
	sh := in.Event.Shell
	if sh == nil || sh.Root == "" {
		return nil
	}
	var paths []string
	for _, word := range shell.WriteTargets(sh.Command, sh.Tool) {
		if p, ok := repoPath(sh.Root, sh.Cwd, word); ok {
			paths = append(paths, p)
		}
	}
	return paths
}

// repoPath places a target word in the repository: joined to the working
// directory unless absolute, then made relative to the root; a path that
// leaves the root is no file of the repository and is refused.
func repoPath(root, cwd, word string) (string, bool) {
	if cwd == "" {
		cwd = root
	}
	p := filepath.FromSlash(word)
	if !filepath.IsAbs(p) {
		p = filepath.Join(cwd, p)
	}
	rel, err := filepath.Rel(root, p)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

package runner

import (
	"slices"
	"strings"
)

// HookVariables names the variables git gives a hook that point a child
// git at the hook's repository and index (section 5.7): a child git with
// GIT_DIR set ignores its working directory, so a test the pre-push rung of
// a linked worktree ran once committed into the worktree's branch.
func HookVariables() []string {
	return []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR", "GIT_PREFIX"}
}

// UserEnv is the environment of a user command a rung spawns: environ
// without the hook variables, names compared without regard to case as
// Windows compares them, so a git the command runs answers for its own
// working directory. The kit's own plumbing keeps environ as it is.
func UserEnv(environ []string) []string {
	hook := HookVariables()
	out := make([]string, 0, len(environ))
	for _, entry := range environ {
		name, _, _ := strings.Cut(entry, "=")
		if !slices.ContainsFunc(hook, func(variable string) bool { return strings.EqualFold(variable, name) }) {
			out = append(out, entry)
		}
	}
	return out
}

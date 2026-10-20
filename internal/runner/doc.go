// Package runner holds the kit's own executions for the git rungs, setup
// and the ladder helpers: the processes they start, the two environments
// of section 5.7 (the hook variables of git kept for the kit's own
// plumbing, stripped for every user command), the pinned toolchain and its
// version diagnosis, the OS file lock, the affected packages, the push
// range and the bounded materializations. The agent hooks never use it:
// they run trusted git plumbing only.
package runner

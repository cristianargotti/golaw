// Package source is the sole owner of snapshots: it discovers the git
// repository, lists the worktree universe through git plumbing, classifies
// every path, reads bytes within the bound and folds the candidates a hook
// supplies into an overlay. Everything stays in memory; no temporary copy
// of the tree is ever written. The index snapshot of staged mode is M2.
package source

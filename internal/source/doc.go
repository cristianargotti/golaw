// Package source is the sole owner of snapshots: it discovers the git
// repository, lists the worktree universe through git plumbing, classifies
// every path, reads bytes within the bound and folds the candidates a hook
// supplies into an overlay; it also freezes the index of the next commit
// and the tree of a commit, whose blobs it reads by object id. Everything
// stays in memory; no temporary copy of the tree is ever written.
package source

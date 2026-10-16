package source

import "context"

// Cache keeps what one process learned from git: each directory is
// discovered once, the root of a repository it found answers the same
// repository, and each repository is listed once. A hook invocation asks
// git each repository fact once through it; a process that must see a
// change on disk builds another.
type Cache struct {
	found map[string]discovery
	trees map[string]*Worktree
}

// discovery is the outcome of one Discover, kept with its error.
type discovery struct {
	repo Repo
	err  error
}

// NewCache answers an empty cache.
func NewCache() *Cache {
	c := &Cache{}
	c.Reset()
	return c
}

// Reset forgets everything the cache learned, so the next question asks
// git again.
func (c *Cache) Reset() {
	c.found, c.trees = map[string]discovery{}, map[string]*Worktree{}
}

// Discover is Discover asked once per directory: a directory asked again,
// or the root of a repository already found, answers what git said then.
func (c *Cache) Discover(ctx context.Context, dir string) (Repo, error) {
	if known, ok := c.found[dir]; ok {
		return known.repo, known.err
	}
	repo, err := Discover(ctx, dir)
	c.found[dir] = discovery{repo: repo, err: err}
	if _, ok := c.found[repo.Root]; err == nil && !ok {
		c.found[repo.Root] = discovery{repo: repo}
	}
	return repo, err
}

// Worktree is the snapshot of a repository listed once: the first call
// lists the universe, and every later call answers a snapshot of the same
// listing that reads each path from disk again, so a content identity
// taken twice compares the bytes of two moments without a second listing.
func (c *Cache) Worktree(ctx context.Context, repo Repo) (*Worktree, error) {
	if listed, ok := c.trees[repo.Root]; ok {
		return listed.Refreshed(), nil
	}
	w, err := NewWorktree(ctx, repo)
	if err != nil {
		return nil, err
	}
	c.trees[repo.Root] = w
	return w, nil
}

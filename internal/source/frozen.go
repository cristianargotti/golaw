package source

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// The modes git records for a symbolic link and for the two kinds of
// regular file; gitlinkMode is the mode of a submodule.
const (
	symlinkMode    = "120000"
	regularMode    = "100644"
	executableMode = "100755"
)

// The names of the empty blob in the two object formats git knows: an
// intent-to-add entry records it, so only an entry that holds it can be
// one, and an index without such an entry asks git nothing about intents.
const (
	emptyBlobSHA1   = "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"
	emptyBlobSHA256 = "473a0f4c3be8a93681a267e3b1e9a7dcda1185436fe141f7749120a303721813"
)

// treeEntry is one path of a frozen universe: its mode and the object id
// git recorded for it.
type treeEntry struct {
	path string
	mode string
	oid  string
}

// stored is what git answered for one object id: its type and size, its
// bytes when they were read whole, or why git could not be asked.
type stored struct {
	info    objectInfo
	content []byte
	read    bool
	failure error
}

// frozen is a universe whose object ids were fixed when it was listed, from
// an index or from the tree of a commit: the bytes of a path are the blob
// of its id, read through git cat-file the first time a caller asks for the
// path, within the read bound, and the same on every later call. Its bytes
// never come from the worktree.
type frozen struct {
	ctx     context.Context //nolint:containedctx // the snapshot interface reads without a context, so a lazy read runs under the one the snapshot was frozen with
	root    string
	env     []string
	entries []treeEntry
	index   map[string]int
	objects map[string]stored
	seen    map[string]model.File
}

// newFrozen holds the entries in path order and reads nothing yet.
func newFrozen(ctx context.Context, root string, env []string, entries []treeEntry) *frozen {
	slices.SortFunc(entries, func(a, b treeEntry) int { return strings.Compare(a.path, b.path) })
	index := make(map[string]int, len(entries))
	for i, e := range entries {
		index[e.path] = i
	}
	return &frozen{
		ctx: ctx, root: root, env: env, entries: entries, index: index,
		objects: map[string]stored{}, seen: map[string]model.File{},
	}
}

// Files lists the universe in path order, every path classified; the
// slice is a copy.
func (f *frozen) Files() []model.File {
	return f.Select(func(string) bool { return true })
}

// Select lists the files whose paths match, in path order, asking git once
// for the sizes and once for the bytes of the blobs not read yet.
func (f *frozen) Select(match func(path string) bool) []model.File {
	picked := f.matching(match)
	f.load(picked)
	out := make([]model.File, 0, len(picked))
	for _, e := range picked {
		out = append(out, f.file(e))
	}
	return out
}

// Prefetch loads the paths that match as Select does, listing nothing, so
// the reads one by one that follow ask git nothing more.
func (f *frozen) Prefetch(match func(path string) bool) {
	f.load(f.matching(match))
}

// Any reports whether a path of the universe matches, reading no blob.
func (f *frozen) Any(match func(path string) bool) bool {
	return slices.ContainsFunc(f.entries, func(e treeEntry) bool { return match(e.path) })
}

// Class answers the class of a path, ClassUnknown outside the universe.
func (f *frozen) Class(rel string) model.FileClass {
	_, file, _ := f.lookup(rel)
	return file.Class
}

// Read returns a copy of the bytes of the blob a path holds. A path outside
// the universe is ErrNotInSnapshot, a blob over the bound ErrTooLarge, and
// a link, a submodule or a blob git cannot read ErrUnreadable; a partial
// blob is never returned.
func (f *frozen) Read(rel string) ([]byte, error) {
	e, file, ok := f.lookup(rel)
	if !ok {
		return nil, fmt.Errorf("%w: %s", model.ErrNotInSnapshot, rel)
	}
	got := f.objects[e.oid]
	switch {
	case regular(e.mode) && got.read:
		return bytes.Clone(got.content), nil
	case regular(e.mode) && got.failure == nil && got.info.kind == blobKind:
		return nil, fmt.Errorf("%w: %s holds %d bytes", model.ErrTooLarge, rel, file.Size)
	default:
		return nil, fmt.Errorf("%w: %s is %s", model.ErrUnreadable, rel, file.Class)
	}
}

// Blob reads any blob of the object store by its id, within the read
// bound, so a check reads the old side of a staged change: a name that is
// not an object id, or one the store does not hold, is ErrNotInSnapshot, an
// object that is not a blob ErrUnreadable, and a blob over the bound
// ErrTooLarge.
func (f *frozen) Blob(id string) ([]byte, error) {
	if !objectID(id) {
		return nil, fmt.Errorf("%w: %q is not an object id", model.ErrNotInSnapshot, id)
	}
	f.load([]treeEntry{{path: id, mode: regularMode, oid: id}})
	got := f.objects[id]
	switch {
	case got.read:
		return bytes.Clone(got.content), nil
	case got.failure != nil:
		return nil, fmt.Errorf("%w: blob %s: %w", model.ErrUnreadable, id, got.failure)
	case got.info.kind == missingKind:
		return nil, fmt.Errorf("%w: blob %s", model.ErrNotInSnapshot, id)
	case got.info.kind != blobKind:
		return nil, fmt.Errorf("%w: %s is a %s", model.ErrUnreadable, id, got.info.kind)
	default:
		return nil, fmt.Errorf("%w: blob %s is %d bytes", model.ErrTooLarge, id, got.info.size)
	}
}

// matching answers the entries whose paths match, in path order.
func (f *frozen) matching(match func(path string) bool) []treeEntry {
	var picked []treeEntry
	for _, e := range f.entries {
		if match(e.path) {
			picked = append(picked, e)
		}
	}
	return picked
}

// lookup answers the entry of a path and its file, its object read first
// when it was not, and whether the path is in the universe.
func (f *frozen) lookup(rel string) (treeEntry, model.File, bool) {
	i, ok := f.index[rel]
	if !ok {
		return treeEntry{}, model.File{}, false
	}
	e := f.entries[i]
	f.load([]treeEntry{e})
	return e, f.file(e), true
}

// load asks git, once for the sizes and once for the bytes, about the
// objects of the entries it has not answered yet: the type and the size of
// every object but a submodule's, whose commit lives in another repository,
// and the bytes of every regular file whose blob fits the read bound. A
// question git fails marks its objects unreadable, so a check reports them
// instead of judging them clean.
func (f *frozen) load(entries []treeEntry) {
	ids := pick(entries, func(e treeEntry) bool {
		_, done := f.objects[e.oid]
		return !done && e.mode != gitlinkMode
	})
	if len(ids) > 0 {
		infos, err := objectInfos(f.ctx, f.root, f.env, ids)
		for _, id := range ids {
			f.objects[id] = stored{info: infos[id], failure: err}
		}
	}
	ids = pick(entries, func(e treeEntry) bool { return regular(e.mode) && f.objects[e.oid].fits() })
	if len(ids) == 0 {
		return
	}
	contents, err := readBlobs(f.ctx, f.root, f.env, ids)
	for _, id := range ids {
		got := f.objects[id]
		got.content, got.read, got.failure = contents[id], err == nil, err
		f.objects[id] = got
	}
}

// file is the class and the size of an entry, from its mode and from what
// git answered for its object: a submodule and a link by their mode, an
// object git could not read unreadable, a blob over the bound by its path
// alone, every other blob by its path and its bytes.
func (f *frozen) file(e treeEntry) model.File {
	if known, ok := f.seen[e.path]; ok {
		return known
	}
	got := f.objects[e.oid]
	file := model.File{Path: e.path, Size: got.info.size}
	switch {
	case e.mode == gitlinkMode:
		file.Class = model.ClassGitlink
	case got.failure != nil || got.info.kind != blobKind:
		file.Class = model.ClassUnreadable
	case e.mode == symlinkMode:
		file.Class = model.ClassSymlink
	case !regular(e.mode):
		file.Class = model.ClassUnreadable
	default:
		file.Class = Classify(e.path, got.content, true)
	}
	f.seen[e.path] = file
	return file
}

// fits reports whether an object is a blob within the read bound whose
// bytes were not read yet.
func (s stored) fits() bool {
	return !s.read && s.failure == nil && s.info.kind == blobKind && s.info.size <= model.MaxReadBytes
}

// pick lists, once each in the order met, the object ids of the entries
// keep accepts.
func pick(entries []treeEntry, keep func(e treeEntry) bool) []string {
	var ids []string
	asked := map[string]bool{}
	for _, e := range entries {
		if !asked[e.oid] && keep(e) {
			asked[e.oid] = true
			ids = append(ids, e.oid)
		}
	}
	return ids
}

// regular reports whether a mode is a regular file, executable or not.
func regular(mode string) bool { return mode == regularMode || mode == executableMode }

// objectID reports whether a name is an object id of one of the two object
// formats: 40 or 64 lower case hexadecimal characters, so a revision
// expression is never handed to git as a blob.
func objectID(id string) bool {
	return (len(id) == len(emptyBlobSHA1) || len(id) == len(emptyBlobSHA256)) && strings.Trim(id, "0123456789abcdef") == ""
}

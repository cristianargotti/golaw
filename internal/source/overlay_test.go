package source_test

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/source"
)

// tree is an in-memory base snapshot: a path maps to its bytes, and a
// class override makes a path a symlink or a binary without bytes.
type tree struct {
	files   map[string]string
	classes map[string]model.FileClass
}

func (b *tree) Files() []model.File {
	out := make([]model.File, 0, len(b.files))
	for rel, content := range b.files {
		out = append(out, model.File{Path: rel, Class: b.Class(rel), Size: int64(len(content))})
	}
	slices.SortFunc(out, func(x, y model.File) int { return cmpPath(x.Path, y.Path) })
	return out
}

func cmpPath(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func (b *tree) Read(rel string) ([]byte, error) {
	content, ok := b.files[rel]
	if !ok {
		return nil, fmt.Errorf("%w: %s", model.ErrNotInSnapshot, rel)
	}
	if !b.Class(rel).IsText() {
		return nil, fmt.Errorf("%w: %s", model.ErrUnreadable, rel)
	}
	return []byte(content), nil
}

func (b *tree) Class(rel string) model.FileClass {
	if c, ok := b.classes[rel]; ok {
		return c
	}
	if _, ok := b.files[rel]; ok {
		return model.ClassTracked
	}
	return model.ClassUnknown
}

func base() *tree {
	return &tree{
		files:   map[string]string{"a.go": "package a\n\nfunc A() {}\n", "docs/x.md": "x\n", "link": "", "bin": "\x00", "link/inner.go": ""},
		classes: map[string]model.FileClass{"link": model.ClassSymlink, "bin": model.ClassBinary},
	}
}

func TestOverlayCreateAndReplace(t *testing.T) {
	t.Parallel()
	muts := []source.Mutation{
		{Path: "./new/b_test.go", Op: model.CandidateCreate, Content: []byte("package b\n")},
		{Path: "a.go", Op: model.CandidateCreate, Content: []byte("package z\n")},
	}
	o, cands, err := source.NewOverlay(base(), muts)
	if err != nil {
		t.Fatal(err)
	}
	want := []model.Candidate{
		{Path: "new/b_test.go", Op: model.CandidateCreate, Reconstructed: true},
		{Path: "a.go", Op: model.CandidateReplace, Reconstructed: true, BaseHash: source.ContentHash([]byte("package a\n\nfunc A() {}\n"))},
	}
	if !slices.Equal(cands, want) {
		t.Errorf("candidates = %+v, want %+v", cands, want)
	}
	if o.Class("new/b_test.go") != model.ClassTest || o.Class("a.go") != model.ClassTracked || o.Class("docs/x.md") != model.ClassTracked {
		t.Errorf("Files() = %+v", o.Files())
	}
	if mustRead(t, o, "a.go") != "package z\n" || mustRead(t, o, "docs/x.md") != "x\n" {
		t.Error("Read must answer the candidate bytes and the base bytes")
	}
	paths := make([]string, 0, len(o.Files()))
	for _, f := range o.Files() {
		paths = append(paths, f.Path)
	}
	if !slices.Equal(paths, []string{"a.go", "bin", "docs/x.md", "link", "link/inner.go", "new/b_test.go"}) {
		t.Errorf("Files() order = %v", paths)
	}
}

func TestOverlayEdit(t *testing.T) {
	t.Parallel()
	hash := source.ContentHash([]byte("package a\n\nfunc A() {}\n"))
	muts := []source.Mutation{{Path: "a.go", Op: model.CandidateEdit, BaseHash: hash, Edits: []source.Edit{
		{Old: "func A() {}", New: "func A() { run() }"},
		{Old: "a", New: "b", All: true},
	}}}
	o, cands, err := source.NewOverlay(base(), muts)
	if err != nil {
		t.Fatal(err)
	}
	if cands[0] != (model.Candidate{Path: "a.go", Op: model.CandidateEdit, Reconstructed: true, BaseHash: hash}) {
		t.Errorf("candidate = %+v", cands[0])
	}
	if got := mustRead(t, o, "a.go"); got != "pbckbge b\n\nfunc A() { run() }\n" {
		t.Errorf("Read = %q", got)
	}
}

func TestOverlayEditNotReconstructed(t *testing.T) {
	t.Parallel()
	hash := source.ContentHash([]byte("package a\n\nfunc A() {}\n"))
	failing := []source.Mutation{{Path: "a.go", Op: model.CandidateEdit, Edits: []source.Edit{{Old: "absent", New: "added text"}}}}
	o, cands, err := source.NewOverlay(base(), failing)
	if err != nil || cands[0].Reconstructed || cands[0].BaseHash != hash {
		t.Fatalf("a failed edit: %+v, %v", cands, err)
	}
	if got := mustRead(t, o, "a.go"); got != "added text" {
		t.Errorf("only the added text is judged: %q", got)
	}
	ambiguous := []source.Mutation{{Path: "a.go", Op: model.CandidateEdit, Edits: []source.Edit{{Old: "a", New: "b"}}}}
	if _, cands, err := source.NewOverlay(base(), ambiguous); err != nil || cands[0].Reconstructed {
		t.Errorf("an ambiguous edit is not reconstructed: %+v, %v", cands, err)
	}
}

func TestOverlayRefusals(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		mut  source.Mutation
		want error
	}{
		"escape":         {source.Mutation{Path: "../x.go", Op: model.CandidateCreate}, source.ErrEscapedRoot},
		"absolute":       {source.Mutation{Path: "/x.go", Op: model.CandidateCreate}, source.ErrEscapedRoot},
		"through link":   {source.Mutation{Path: "link/inner.go", Op: model.CandidateCreate}, source.ErrUnsafeLink},
		"is link":        {source.Mutation{Path: "link", Op: model.CandidateDelete}, source.ErrUnsafeLink},
		"stale base":     {source.Mutation{Path: "a.go", Op: model.CandidateReplace, BaseHash: "0"}, source.ErrBaseMismatch},
		"stale edit":     {source.Mutation{Path: "a.go", Op: model.CandidateEdit, BaseHash: "0", Edits: []source.Edit{{Old: "a", New: "b"}}}, source.ErrBaseMismatch},
		"edit absent":    {source.Mutation{Path: "gone.go", Op: model.CandidateEdit, Edits: []source.Edit{{Old: "a", New: "b"}}}, source.ErrOverlay},
		"edit binary":    {source.Mutation{Path: "bin", Op: model.CandidateEdit, Edits: []source.Edit{{Old: "a", New: "b"}}}, source.ErrOverlay},
		"edit nothing":   {source.Mutation{Path: "a.go", Op: model.CandidateEdit}, source.ErrOverlay},
		"delete absent":  {source.Mutation{Path: "gone.go", Op: model.CandidateDelete}, source.ErrOverlay},
		"move absent":    {source.Mutation{Path: "b.go", Op: model.CandidateMove, From: "gone.go"}, source.ErrOverlay},
		"move from link": {source.Mutation{Path: "b.go", Op: model.CandidateMove, From: "link"}, source.ErrUnsafeLink},
		"unknown op":     {source.Mutation{Path: "a.go", Op: 0}, source.ErrOverlay},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, _, err := source.NewOverlay(base(), []source.Mutation{c.mut}); !errors.Is(err, c.want) {
				t.Errorf("NewOverlay = %v, want %v", err, c.want)
			}
		})
	}
}

func TestOverlayDeleteAndMove(t *testing.T) {
	t.Parallel()
	muts := []source.Mutation{
		{Path: "docs/x.md", Op: model.CandidateDelete},
		{Path: "b.go", Op: model.CandidateMove, From: "a.go", Edits: []source.Edit{{Old: "package a", New: "package b"}}},
		{Path: "bin2", Op: model.CandidateMove, From: "bin"},
	}
	o, cands, err := source.NewOverlay(base(), muts)
	if err != nil {
		t.Fatal(err)
	}
	if cands[0].Op != model.CandidateDelete || cands[1].From != "a.go" || !cands[1].Reconstructed || cands[2].Op != model.CandidateMove {
		t.Errorf("candidates = %+v", cands)
	}
	for _, gone := range []string{"docs/x.md", "a.go", "bin"} {
		if o.Class(gone) != model.ClassUnknown || !errors.Is(readErr(o, gone), model.ErrNotInSnapshot) {
			t.Errorf("%s must leave the universe", gone)
		}
	}
	if got := mustRead(t, o, "b.go"); got != "package b\n\nfunc A() {}\n" {
		t.Errorf("Read(b.go) = %q", got)
	}
	if o.Class("bin2") != model.ClassBinary || !errors.Is(readErr(o, "bin2"), model.ErrUnreadable) {
		t.Errorf("a moved binary keeps its class and its read error: %s", o.Class("bin2"))
	}
	sizes := map[string]int64{}
	for _, f := range o.Files() {
		sizes[f.Path] = f.Size
	}
	if sizes["b.go"] != 23 || sizes["bin2"] != 1 {
		t.Errorf("sizes = %v", sizes)
	}
}

func TestOverlayMoveNotReconstructed(t *testing.T) {
	t.Parallel()
	failing := []source.Mutation{{Path: "b.go", Op: model.CandidateMove, From: "a.go", Edits: []source.Edit{{Old: "zzz", New: "added"}}}}
	o, cands, err := source.NewOverlay(base(), failing)
	if err != nil || cands[0].Reconstructed {
		t.Fatalf("a move whose edits fail is not reconstructed: %+v, %v", cands, err)
	}
	if got := mustRead(t, o, "b.go"); got != "added" {
		t.Errorf("only the added text is judged: %q", got)
	}
}

func TestOverlayReadBound(t *testing.T) {
	t.Parallel()
	muts := []source.Mutation{
		{Path: "at.bin", Op: model.CandidateCreate, Content: []byte(bulk(model.MaxReadBytes))},
		{Path: "over.bin", Op: model.CandidateCreate, Content: []byte(bulk(model.MaxReadBytes + 1))},
	}
	o, _, err := source.NewOverlay(base(), muts)
	if err != nil {
		t.Fatal(err)
	}
	if got := mustRead(t, o, "at.bin"); len(got) != model.MaxReadBytes {
		t.Errorf("a candidate at the bound reads whole: %d bytes", len(got))
	}
	if err := readErr(o, "over.bin"); !errors.Is(err, model.ErrTooLarge) {
		t.Errorf("a candidate past the bound: %v, want ErrTooLarge", err)
	}
}

func TestOverlayOverWorktree(t *testing.T) {
	t.Parallel()
	root := initRepo(t)
	write(t, root, "a.go", "package a\n")
	commitAll(t, root, "one")
	repo, err := source.Discover(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	w, err := source.NewWorktree(t.Context(), repo)
	if err != nil {
		t.Fatal(err)
	}
	rel, err := source.Rel(repo.Root, repo.Root, "new.go")
	if err != nil {
		t.Fatal(err)
	}
	o, cands, err := source.NewOverlay(w, []source.Mutation{{Path: rel, Op: model.CandidateReplace, Content: []byte("package n\n")}})
	if err != nil || cands[0].Op != model.CandidateCreate {
		t.Fatalf("a candidate absent from disk: %+v, %v", cands, err)
	}
	if o.Class("new.go") != model.ClassUntracked || o.Class("a.go") != model.ClassTracked {
		t.Errorf("Files() = %+v", o.Files())
	}
}

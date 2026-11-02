package genmap

import (
	"cmp"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/holes"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/genheader"
	"github.com/cristianargotti/golaw/internal/model"
)

// DefaultBudget is the size of the map in tokens (A52); a token is
// estimated as four bytes, so the text stays within what a session start
// context can spend on it.
const DefaultBudget = 2000

// Input is what one map reads.
type Input struct {
	Snapshot model.Snapshot       // the repository as it is
	Manifest *manifest.Manifest   // nil when the repository has none yet
	Policy   model.GeneratePolicy // the layout the map places files with
	Area     string               // keep one area, empty for the whole map
}

// Item is one port, use case, adapter or handler of the map.
type Item struct {
	Name    string   // "<area or package>.<Type>"
	Path    string   // the file that declares or holds it
	Details []string // "methods A, B", "port growth.Ledger", "in X"
	Note    string   // "handwritten: <reason>" for a marked file
}

// Report is the whole map, every section sorted.
type Report struct {
	Layers    []string     // "<kind> <directory pattern>" in layout order
	Holes     []holes.Hole // every open hole, by path then line
	Ports     []Item
	Usecases  []Item
	Adapters  []Item
	Handlers  []Item
	Contracts []string // the contract paths, sorted and distinct
	// Handwritten is every other Go file marked golaw:handwritten, its
	// reason as the note, so a marked file no section holds still shows.
	Handwritten []Item
}

// markHandwritten notes every item whose file took the escape, so the
// agent knows golaw never generates into it (M7.9.5).
func markHandwritten(r *Report, files []goFile) {
	for _, section := range [][]Item{r.Ports, r.Usecases, r.Adapters, r.Handlers} {
		for i := range section {
			j := slices.IndexFunc(files, func(f goFile) bool { return f.path == section[i].Path })
			if j < 0 {
				continue
			}
			if _, reason, marked := genheader.HandwrittenAt(files[j].content); marked {
				section[i].Note = "handwritten without a reason"
				if reason != "" {
					section[i].Note = "handwritten: " + reason
				}
			}
		}
	}
}

// listHandwritten adds every marked source no item names, by path.
func listHandwritten(r *Report, files []goFile) {
	named := map[string]bool{}
	for _, section := range [][]Item{r.Ports, r.Usecases, r.Adapters, r.Handlers} {
		for _, it := range section {
			named[it.Path] = true
		}
	}
	for _, f := range files {
		if _, reason, marked := genheader.HandwrittenAt(f.content); marked && !f.test && !named[f.path] {
			r.Handwritten = append(r.Handwritten, Item{Name: f.path, Path: f.path, Note: reason})
		}
	}
	slices.SortFunc(r.Handwritten, func(a, b Item) int { return cmp.Compare(a.Path, b.Path) })
}

// sortReport orders every section by name then path and the holes by path
// then line, so two runs over one tree print the same map.
func sortReport(r *Report) {
	byName := func(a, b Item) int { return cmp.Or(cmp.Compare(a.Name, b.Name), cmp.Compare(a.Path, b.Path)) }
	for _, section := range [][]Item{r.Ports, r.Usecases, r.Adapters, r.Handlers} {
		slices.SortFunc(section, byName)
	}
	slices.SortFunc(r.Holes, func(a, b holes.Hole) int { return cmp.Or(cmp.Compare(a.Path, b.Path), cmp.Compare(a.Line, b.Line)) })
}

// portItem is the map item of one parsed interface.
func portItem(area, file, name string, methods []genkind.Method) Item {
	it := Item{Name: area + "." + name, Path: file}
	if len(methods) > 0 {
		names := make([]string, 0, len(methods))
		for _, m := range methods {
			names = append(names, m.Name)
		}
		it.Details = []string{"methods " + strings.Join(names, ", ")}
	}
	return it
}

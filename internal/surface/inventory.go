package surface

import (
	"context"
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// Inventory is the non-Go surface of a snapshot, computed on every run and
// never stored (A25).
type Inventory struct {
	Files      []FileInfo // every surface file of the snapshot, in path order
	Languages  []Language // the languages a judge reads, in Languages() order
	Unmeasured []string   // "path: reason" for each file Detect could not judge
}

// Take inventories the text files of a snapshot. It reads only the files
// whose language ByName leaves to the content, so a name-settled file
// carries its name class, the snapshot's generated class, and no regions:
// a caller that judges content calls Detect on the bytes it reads. A
// language counts as present only through a source or declaration file: a
// language that only fixtures, generated or minified files hold needs no
// tool, and naming it would send a person after one.
func Take(ctx context.Context, s model.Snapshot) (Inventory, error) {
	var inv Inventory
	present := map[Language]bool{}
	files := slices.Clone(s.Files())
	slices.SortFunc(files, func(a, b model.File) int { return strings.Compare(a.Path, b.Path) })
	for _, f := range files {
		if err := ctx.Err(); err != nil {
			return Inventory{}, fmt.Errorf("take the surface inventory: %w", err)
		}
		info, cause := entry(s, f)
		if cause != "" {
			inv.Unmeasured = append(inv.Unmeasured, f.Path+": "+cause)
		}
		if info.Language != "" {
			inv.Files = append(inv.Files, info)
			present[info.Language] = present[info.Language] || info.Class == ClassSource || info.Class == ClassDeclaration
		}
	}
	for _, l := range Languages() {
		if present[l] {
			inv.Languages = append(inv.Languages, l)
		}
	}
	return inv, nil
}

// entry detects one file of the snapshot and answers the cause it could
// not be judged, empty when it was.
func entry(s model.Snapshot, f model.File) (FileInfo, string) {
	if !f.Class.IsText() {
		return FileInfo{}, ""
	}
	language, settled := ByName(f.Path)
	if settled {
		if language == "" {
			return FileInfo{}, ""
		}
		info := FileInfo{Path: f.Path, Language: language, Class: classOf(f.Path, language, nil)}
		if f.Class == model.ClassGenerated {
			info.Class = ClassGenerated
		}
		return info, ""
	}
	content, err := s.Read(f.Path)
	if errors.Is(err, model.ErrTooLarge) {
		return pastBound(f.Path), ""
	}
	if err != nil {
		return FileInfo{}, err.Error()
	}
	info, err := Detect(f.Path, content)
	if err != nil {
		return info, err.Error()
	}
	return info, ""
}

// pastBound settles a file larger than the read bound by its name alone:
// a CloudFormation template holds at most 1 MB and no script is that
// large, so a lock file or a data dump past the bound is data, and a
// false Unmeasured would fail the lint target in CI.
func pastBound(name string) FileInfo {
	if yamlExt(strings.ToLower(path.Ext(name))) {
		return FileInfo{Path: name, Language: YAML, Class: classOf(name, YAML, nil)}
	}
	return FileInfo{}
}

// Has reports whether the inventory holds a language.
func (inv Inventory) Has(l Language) bool {
	return slices.Contains(inv.Languages, l)
}

// Of answers the files of one language, in path order.
func (inv Inventory) Of(l Language) []FileInfo {
	var out []FileInfo
	for _, f := range inv.Files {
		if f.Language == l {
			out = append(out, f)
		}
	}
	return out
}

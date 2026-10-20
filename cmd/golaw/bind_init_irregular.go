package main

import (
	"slices"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/bootstrap"
)

// filterInitIrregular leaves out the destinations a symbolic link or a
// component of another file type holds, so one such path never stops an
// adoption; inspected says which destinations the operation would read.
func filterInitIrregular(root string, rendered []assets.Rendered, inspected func(string) bool) ([]assets.Rendered, []string, error) {
	paths := make([]string, 0, len(rendered))
	for i := range rendered {
		if inspected(rendered[i].Path) {
			paths = append(paths, rendered[i].Path)
		}
	}
	irregular, err := bootstrap.IrregularPaths(root, paths)
	if err != nil {
		return nil, nil, initOperation("inspect template destinations", err)
	}
	kept := slices.DeleteFunc(slices.Clone(rendered), func(file assets.Rendered) bool {
		return slices.Contains(irregular, file.Path)
	})
	return kept, irregular, nil
}

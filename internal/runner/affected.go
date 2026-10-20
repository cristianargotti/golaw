package runner

import (
	"context"
	"path"
	"slices"
	"strings"
)

// AffectedSet keeps concrete packages even when module changes require all patterns.
type AffectedSet struct {
	All      bool
	Packages []string
	Patterns []string
}

// Affected includes reverse test importers so changed dependencies cannot evade checks.
func Affected(ctx context.Context, root string, paths, env []string) (AffectedSet, error) {
	if len(paths) == 0 {
		return AffectedSet{}, nil
	}
	graph, err := listPackages(ctx, root, env)
	if err != nil {
		return AffectedSet{}, err
	}
	seeds, all := graph.seeds(paths)
	return graph.selected(graph.closure(seeds), all), nil
}

func (g *packageGraph) seeds(paths []string) ([]string, bool) {
	var seeds []string
	for _, changed := range paths {
		if changed == "" {
			continue
		}
		changed = path.Clean(changed)
		if path.IsAbs(changed) || changed == ".." || strings.HasPrefix(changed, "../") {
			continue
		}
		if changed == "go.mod" || changed == "go.sum" {
			return nil, true
		}
		dir := changedDirectory(changed)
		identity := g.module
		if dir != "." {
			identity += "/" + dir
		}
		_, present := g.packages[identity]
		if present || len(g.reverse[identity]) > 0 {
			seeds = append(seeds, identity)
		}
	}
	return seeds, false
}

func changedDirectory(changed string) string {
	parts := strings.Split(changed, "/")
	for i, part := range parts {
		if part == "testdata" {
			return path.Clean(strings.Join(parts[:i], "/"))
		}
	}
	return path.Dir(changed)
}

func (g *packageGraph) selected(closure map[string]bool, all bool) AffectedSet {
	result := AffectedSet{All: all}
	for identity, dir := range g.packages {
		if !all && !closure[identity] {
			continue
		}
		result.Packages = append(result.Packages, identity)
		pattern := "."
		if dir != "" {
			pattern = "./" + dir
		}
		result.Patterns = append(result.Patterns, pattern)
	}
	slices.Sort(result.Packages)
	slices.Sort(result.Patterns)
	if all {
		result.Patterns = []string{"./..."}
	}
	return result
}

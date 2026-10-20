package runner

import (
	"fmt"
	"strings"
)

func (g *packageGraph) connect(records []listedPackage) error {
	for i := range records {
		record := &records[i]
		if record.testExecutable() {
			continue
		}
		owner := g.aliases[record.ImportPath]
		if owner == "" {
			continue
		}
		if _, exists := g.packages[owner]; !exists {
			return fmt.Errorf("%w: test variant has no canonical package %q", ErrPackages, owner)
		}
		if _, exists := g.packages[record.ForTest]; record.ForTest != "" && !exists {
			return fmt.Errorf("%w: test variant has no owner %q", ErrPackages, record.ForTest)
		}
		for _, imports := range [][]string{record.Imports, record.TestImports, record.XTestImports} {
			if err := g.connectImports(owner, imports); err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *packageGraph) connectImports(owner string, imports []string) error {
	for _, imported := range imports {
		identity, err := g.importedIdentity(imported)
		if err != nil {
			return err
		}
		if identity == "" || identity == owner {
			continue
		}
		if g.reverse[identity] == nil {
			g.reverse[identity] = make(map[string]bool)
		}
		g.reverse[identity][owner] = true
	}
	return nil
}

func (g *packageGraph) importedIdentity(imported string) (string, error) {
	if identity, known := g.aliases[imported]; known {
		return identity, nil
	}
	identity, decoration, decorated := strings.Cut(imported, " [")
	if !importIdentity(identity) {
		if !decorated {
			return "", nil
		}
		return "", fmt.Errorf("%w: invalid import %q", ErrPackages, imported)
	}
	if !g.contains(identity) {
		return "", nil
	}
	if decorated {
		_, known := g.packages[identity]
		owner := strings.TrimSuffix(decoration, ".test]")
		_, testKnown := g.packages[owner]
		if !known || !testKnown || owner == decoration || !importIdentity(owner) {
			return "", fmt.Errorf("%w: unresolved test import %q", ErrPackages, imported)
		}
	}
	return identity, nil
}

func (g *packageGraph) closure(seeds []string) map[string]bool {
	seen := make(map[string]bool)
	for len(seeds) > 0 {
		identity := seeds[0]
		seeds = seeds[1:]
		if seen[identity] {
			continue
		}
		seen[identity] = true
		for importer := range g.reverse[identity] {
			seeds = append(seeds, importer)
		}
	}
	return seen
}

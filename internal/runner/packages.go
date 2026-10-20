package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
)

type listedModule struct {
	Path string `json:"Path"`
	Main bool   `json:"Main"`
}

type listedPackage struct {
	ImportPath   string        `json:"ImportPath"`
	Name         string        `json:"Name"`
	ForTest      string        `json:"ForTest"`
	Module       *listedModule `json:"Module"`
	Match        []string      `json:"Match"`
	Imports      []string      `json:"Imports"`
	TestImports  []string      `json:"TestImports"`
	XTestImports []string      `json:"XTestImports"`
}

type packageGraph struct {
	module   string
	packages map[string]string
	aliases  map[string]string
	reverse  map[string]map[string]bool
}

func listPackages(ctx context.Context, root string, env []string) (packageGraph, error) {
	var records []listedPackage
	consume := func(reader io.Reader) error {
		var err error
		records, err = decodePackages(reader)
		return err
	}
	out, err := ReadOutput(ctx, Command{
		Name: "go", Args: []string{"list", "-e", "-test", "-json", "./..."},
		Dir: root, Env: env,
	}, consume)
	if err != nil {
		return packageGraph{}, fmt.Errorf("%w: list packages: %w", ErrPackages, err)
	}
	if out.Code != 0 {
		return packageGraph{}, fmt.Errorf("%w: go list exited %d: %s", ErrPackages, out.Code, out.Stderr)
	}
	return buildPackageGraph(records)
}

func decodePackages(reader io.Reader) ([]listedPackage, error) {
	var records []listedPackage
	decoder := json.NewDecoder(reader)
	for {
		var record listedPackage
		err := decoder.Decode(&record)
		if errors.Is(err, io.EOF) {
			return records, nil
		}
		if err != nil {
			return nil, fmt.Errorf("decode package: %w", err)
		}
		records = append(records, record)
	}
}

func buildPackageGraph(records []listedPackage) (packageGraph, error) {
	module, err := mainModule(records)
	if err != nil {
		return packageGraph{}, err
	}
	graph := packageGraph{
		module: module, packages: make(map[string]string),
		aliases: make(map[string]string), reverse: make(map[string]map[string]bool),
	}
	for i := range records {
		if err := graph.identify(records[i]); err != nil {
			return packageGraph{}, err
		}
	}
	if err := graph.connect(records); err != nil {
		return packageGraph{}, err
	}
	return graph, nil
}

func mainModule(records []listedPackage) (string, error) {
	var module string
	for i := range records {
		record := &records[i]
		if record.Module == nil || !record.Module.Main {
			continue
		}
		name := record.Module.Path
		if !importIdentity(name) || (module != "" && module != name) {
			return "", fmt.Errorf("%w: inconsistent main module %q", ErrPackages, name)
		}
		module = name
	}
	if module == "" && len(records) != 0 {
		return "", fmt.Errorf("%w: package listing has no main module", ErrPackages)
	}
	return module, nil
}

func (g *packageGraph) identify(record listedPackage) error {
	identity, err := packageIdentity(record)
	if err != nil {
		return err
	}
	if record.testExecutable() {
		return nil
	}
	if record.Module == nil {
		if g.contains(identity) {
			return fmt.Errorf("%w: missing module for %q", ErrPackages, identity)
		}
		return nil
	}
	if !record.Module.Main {
		g.aliases[record.ImportPath] = ""
		return nil
	}
	if !g.contains(identity) {
		return fmt.Errorf("%w: package %q is outside its module", ErrPackages, identity)
	}
	g.aliases[record.ImportPath] = identity
	if record.ForTest == "" {
		g.packages[identity] = strings.TrimPrefix(strings.TrimPrefix(identity, g.module), "/")
	}
	return nil
}

func (p *listedPackage) testExecutable() bool {
	// A real main package ending in .test still carries its matching pattern.
	return p.ForTest == "" && p.Name == "main" &&
		strings.HasSuffix(p.ImportPath, ".test") && len(p.Match) == 0
}

func packageIdentity(record listedPackage) (string, error) {
	identity, decoration, decorated := strings.Cut(record.ImportPath, " [")
	if !importIdentity(identity) || (decorated && (record.ForTest == "" || decoration != record.ForTest+".test]")) {
		return "", fmt.Errorf("%w: invalid package identity %q", ErrPackages, record.ImportPath)
	}
	if record.ForTest != "" && !importIdentity(record.ForTest) {
		return "", fmt.Errorf("%w: invalid test owner %q", ErrPackages, record.ForTest)
	}
	if record.ForTest != "" && identity == record.ForTest+"_test" {
		identity = record.ForTest
	}
	return identity, nil
}

func importIdentity(name string) bool {
	return name != "" && name != "." && name != ".." &&
		!strings.HasPrefix(name, "/") && !strings.HasPrefix(name, "../") &&
		!strings.ContainsAny(name, " \\[]\t\r\n") && path.Clean(name) == name
}

func (g *packageGraph) contains(identity string) bool {
	return identity == g.module || strings.HasPrefix(identity, g.module+"/")
}

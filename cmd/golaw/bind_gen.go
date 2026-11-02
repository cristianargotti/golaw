package main

import (
	"context"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"path"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/gen"
	"github.com/cristianargotti/golaw/internal/gen/contract"
	"github.com/cristianargotti/golaw/internal/gen/kinds"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/newcode"
)

// genRun is what the commands of the engine read once per run: the
// repository root the writes land under, the policy in force, the frozen
// worktree and the engine input built over them, so a plan and its report
// never read the tree twice.
type genRun struct {
	root     string
	policy   *model.EffectivePolicy
	snapshot model.Snapshot
	input    gen.Input
}

// prepareGen reads the repository that holds dir into one engine input.
func (a *app) prepareGen(ctx context.Context, dir string) (genRun, error) {
	root, err := a.discover(ctx, dir)
	if err != nil {
		return genRun{}, fmt.Errorf("find the repository: %w", err)
	}
	policy, err := a.load(ctx, root, a.now())
	if err != nil {
		return genRun{}, fmt.Errorf("load the policy: %w", err)
	}
	snapshot, err := a.snapshot(ctx, root)
	if err != nil {
		return genRun{}, fmt.Errorf("read the worktree: %w", err)
	}
	module, err := moduleOf(snapshot)
	if err != nil {
		return genRun{}, fmt.Errorf("read the module: %w", err)
	}
	m, err := manifestOf(snapshot)
	if err != nil {
		return genRun{}, fmt.Errorf("read the manifest: %w", err)
	}
	main, err := mainOf(policy, snapshot, module)
	if err != nil {
		return genRun{}, fmt.Errorf("find the main package: %w", err)
	}
	input := gen.Input{
		Module: module, Version: pinOf(a.buildInfo()), Policy: policy.Generate,
		Snapshot: snapshot, Manifest: m, Contracts: contract.Reader{Snapshot: snapshot},
		Kinds: kinds.All(), Main: main,
	}
	return genRun{root: root, policy: policy, snapshot: snapshot, input: input}, nil
}

// moduleOf answers the module path of the go.mod at the root.
func moduleOf(s model.Snapshot) (string, error) {
	data, err := s.Read(initModuleFile)
	if errors.Is(err, model.ErrNotInSnapshot) {
		return "", fmt.Errorf("%w: go.mod is absent: the generators write into a Go module", model.ErrUsage)
	}
	if err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}
	module, err := newcode.Module(data)
	if err != nil {
		return "", fmt.Errorf("%w: go.mod: %w", model.ErrUsage, err)
	}
	return module, nil
}

// manifestOf answers the generation manifest, an empty one before the
// first golaw new, because the first run is what creates it.
func manifestOf(s model.Snapshot) (manifest.Manifest, error) {
	m, err := readManifest(s)
	if err != nil {
		return manifest.Manifest{}, err
	}
	if m == nil {
		return manifest.Manifest{Schema: manifest.Schema}, nil
	}
	return *m, nil
}

// mainOf answers the Main of the wiring: empty for a root main package, a
// cmd directory name, or gen.MainByHand when the composition roots are
// written by hand under cmd, because the wiring of phase M7.1 composes one
// root and must never write a second one beside them.
func mainOf(p *model.EffectivePolicy, s model.Snapshot, module string) (string, error) {
	found, err := mainPackages(s)
	if err != nil {
		return "", err
	}
	library := path.Base(module)
	if p.Layout.Main == bootstrap.MainCmd {
		switch len(found.mains) {
		case 0:
			return library, nil
		case 1:
			return found.mains[0], nil
		default:
			return gen.MainByHand, nil
		}
	}
	switch {
	case found.rootMain || (!found.rootGo && len(found.mains) == 0):
		return "", nil
	case !found.rootGo:
		return gen.MainByHand, nil
	default:
		return library, nil
	}
}

// layoutMains is what mainOf reads of the tree: whether the root holds Go,
// whether that Go is package main, and the cmd directories that are.
type layoutMains struct {
	rootGo, rootMain bool
	mains            []string
}

// mainPackages reads the package clause of every non test Go file at the
// root and directly under a cmd directory.
func mainPackages(s model.Snapshot) (layoutMains, error) {
	var found layoutMains
	for _, f := range s.Files() {
		dir := path.Dir(f.Path)
		atRoot, underCmd := dir == ".", path.Dir(dir) == "cmd"
		if (!atRoot && !underCmd) || !strings.HasSuffix(f.Path, ".go") || strings.HasSuffix(f.Path, "_test.go") {
			continue
		}
		isMain, clauseErr := packageMain(s, f.Path)
		if clauseErr != nil {
			return layoutMains{}, clauseErr
		}
		switch {
		case atRoot:
			found.rootGo, found.rootMain = true, found.rootMain || isMain
		case isMain && !slices.Contains(found.mains, path.Base(dir)):
			found.mains = append(found.mains, path.Base(dir))
		}
	}
	slices.Sort(found.mains)
	return found, nil
}

// packageMain reports whether the file's package clause names main.
func packageMain(s model.Snapshot, rel string) (bool, error) {
	content, err := s.Read(rel)
	if err != nil {
		return false, fmt.Errorf("read the package clause of %s: %w", rel, err)
	}
	f, err := parser.ParseFile(token.NewFileSet(), rel, content, parser.PackageClauseOnly)
	if err != nil {
		return false, fmt.Errorf("%w: %s: the package clause does not parse: %w", model.ErrUsage, rel, err)
	}
	return f.Name.Name == "main", nil
}

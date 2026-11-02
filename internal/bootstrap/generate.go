package bootstrap

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/cristianargotti/golaw/internal/genheader"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/source"
)

// adaptersRoot is the candidate root whose children name the adapter
// layouts a repository may hold (M7.6.2).
const adaptersRoot = "internal/adapters"

// packageSlot ends an adapter or handler pattern: one directory per package.
const packageSlot = "/{package}"

// side is one kind init places: the key the person sets when the tree is
// ambiguous and the directory names under adaptersRoot that may hold it.
type side struct {
	key   string
	names []string
}

// DetectGenerate answers the generate block of a new policy for the tree
// at root: the adapter and handler layouts the hand written packages
// already use, the defaults where the tree holds none, and ErrLayout
// when two layouts both hold hand written packages for one kind. The
// port, use case, wiring, support and OpenAPI keys keep their defaults:
// their candidate roots carry no alternative in phase M7.1.
func DetectGenerate(root string) (model.GeneratePolicy, error) {
	g := model.DefaultGenerate()
	g.Declared = true
	out, err := pick(root, side{"generate.layout.adapter_sql and generate.layout.adapter_http", []string{"out", "outbound"}})
	if err != nil {
		return model.GeneratePolicy{}, err
	}
	if out != "" {
		base := adaptersRoot + "/" + out
		g.Layout.AdapterSQL = base + packageSlot
		g.Layout.AdapterHTTP = base + packageSlot
		g.ThirdParty.SQLCOut = base + "/postgres/sqlcgen"
	}
	in, err := pick(root, side{"generate.layout.handler", []string{"in", "inbound"}})
	if err != nil {
		return model.GeneratePolicy{}, err
	}
	if in != "" {
		g.Layout.Handler = adaptersRoot + "/" + in + packageSlot
	}
	return g, nil
}

// pick answers the one directory of s that holds hand written packages,
// empty when none does; two are never a silent choice.
func pick(root string, s side) (string, error) {
	var found []string
	counts := make(map[string]int, len(s.names))
	for _, name := range s.names {
		n, err := handwrittenPackages(root, adaptersRoot+"/"+name)
		if err != nil {
			return "", err
		}
		if n > 0 {
			found = append(found, name)
			counts[name] = n
		}
	}
	switch len(found) {
	case 0:
		return "", nil
	case 1:
		return found[0], nil
	default:
		return "", fmt.Errorf("%w: %s/%s (%d packages) and %s/%s (%d packages) both hold hand written adapters: set %s in %s",
			ErrLayout, adaptersRoot, found[0], counts[found[0]], adaptersRoot, found[1], counts[found[1]], s.key, PolicyFile)
	}
}

// handwrittenPackages counts the directories under dir that hold a Go
// file a person wrote; a directory of generator output alone is an output
// path, not a kind path, and a symbolic link is never followed, so the
// detection stays inside the root.
func handwrittenPackages(root, dir string) (int, error) {
	packages := make(map[string]bool)
	top := filepath.Join(root, filepath.FromSlash(dir))
	err := filepath.WalkDir(top, func(path string, d fs.DirEntry, err error) error {
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return nil
		case err != nil:
			return err
		case d.IsDir():
			return skipImportDirectory(top, path, d.Name())
		case d.Type()&fs.ModeSymlink != 0 || packages[filepath.Dir(path)]:
			return nil
		}
		written, err := handwrittenGo(root, path)
		packages[filepath.Dir(path)] = written
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("detect the layout under %s: %w", dir, err)
	}
	n := 0
	for _, written := range packages {
		if written {
			n++
		}
	}
	return n, nil
}

// handwrittenGo reports a Go file outside the tests that carries neither a
// generated header nor the golaw one.
func handwrittenGo(root, path string) (bool, error) {
	if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
		return false, nil
	}
	f, err := os.Open(path) //nolint:gosec // The walk stays under the resolved root and follows no link.
	if err != nil {
		return false, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	content, err := io.ReadAll(io.LimitReader(f, model.MaxReadBytes))
	if err != nil {
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false, fmt.Errorf("relate %s: %w", path, err)
	}
	if _, golaw := genheader.Parse(content); golaw {
		return false, nil
	}
	return source.Classify(filepath.ToSlash(rel), content, true) != model.ClassGenerated, nil
}

// block is the generate block of M7.6.2 in its key order, one plain
// scalar per key.
func block(g model.GeneratePolicy) string {
	return "generate:\n" +
		"  layout:\n" +
		"    port: " + g.Layout.Port + "\n" +
		"    usecase: " + g.Layout.Usecase + "\n" +
		"    adapter_sql: " + g.Layout.AdapterSQL + "\n" +
		"    adapter_http: " + g.Layout.AdapterHTTP + "\n" +
		"    handler: " + g.Layout.Handler + "\n" +
		"    wiring: " + g.Layout.Wiring + "\n" +
		"  support: " + g.Support + "\n" +
		"  third_party:\n" +
		"    sqlc_out: " + g.ThirdParty.SQLCOut + "\n" +
		"    openapi: " + g.ThirdParty.OpenAPI + "\n" +
		"    openapi_out: " + g.ThirdParty.OpenAPIOut + "\n"
}

// SetGenerate answers a policy with the generate block g appended; a file
// that already names a block comes back unchanged, because a reviewed
// block is the person's. A block that would not read back as g is left
// to the person with the block to write.
func SetGenerate(current []byte, g model.GeneratePolicy) ([]byte, error) {
	config, err := policy.Decode(current)
	if err != nil {
		return nil, fmt.Errorf("read the generate block of %s: %w", PolicyFile, err)
	}
	if config.Generate != nil {
		return current, nil
	}
	text := block(g)
	proposed := appendLine(current, strings.TrimSuffix(text, "\n"))
	if config, err = policy.Decode(proposed); err == nil && config.Generate.Resolve() == g {
		return proposed, nil
	}
	return nil, fmt.Errorf("%w: %s does not take the generate block init detected; write it by hand:\n%s",
		model.ErrUsage, PolicyFile, text)
}

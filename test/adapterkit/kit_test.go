package adapterkit_test

import (
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/contract"
	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/kinds/adapterkit"
)

const kitModule = "github.com/cristianargotti/golaw"

// TestMethodsShapeOrRefuseEveryInterfaceOfTheKit shapes every interface of
// the kit's own source, a tree of real ports in every style: a method either
// becomes a declaration that parses and imports exactly what it names, or is
// refused with a sentinel the command maps to an exit code. Any other answer
// would be generated code that does not build, or a refusal without its fix.
func TestMethodsShapeOrRefuseEveryInterfaceOfTheKit(t *testing.T) {
	t.Parallel()
	shaped, refused := 0, 0
	for _, port := range kitPorts(t, filepath.Join("..", "..")) {
		for _, m := range port.Methods {
			one := port
			one.Methods = []genkind.Method{m}
			target := adapterkit.Target{
				Name: genkind.Name{Area: "probe", Type: "Probe"}, Port: one, Recv: "p",
				PortPkg: genkind.Import{Path: kitModule + "/" + port.Dir},
			}
			if port.Package != path.Base(port.Dir) {
				target.PortPkg.Name = port.Package
			}
			methods, imports, err := adapterkit.Methods(target)
			if err != nil {
				refused++
				requireRefusal(t, port, m, err)
				continue
			}
			shaped++
			requireBuilds(t, port, methods[0], imports)
		}
	}
	if shaped == 0 || refused == 0 {
		t.Fatalf("shaped %d and refused %d methods: the kit holds both, the walk lost its root", shaped, refused)
	}
}

func requireRefusal(t *testing.T, port genkind.Port, m genkind.Method, err error) {
	t.Helper()
	if !errors.Is(err, genkind.ErrRequest) && !errors.Is(err, genkind.ErrCannotJudge) {
		t.Errorf("%s %s.%s: refused without a sentinel: %v", port.Path, port.Name, m.Name, err)
	}
	if !strings.Contains(err.Error(), m.Name) && !errors.Is(err, genkind.ErrCannotJudge) {
		t.Errorf("%s %s.%s: the refusal does not name the method: %v", port.Path, port.Name, m.Name, err)
	}
}

// requireBuilds parses the method as the adapter would declare it and holds
// the imports to the ones its types select.
func requireBuilds(t *testing.T, port genkind.Port, m adapterkit.Method, imports []genkind.Import) {
	t.Helper()
	src := fmt.Sprintf("package probe\n\nfunc (p *Probe) %s%s {\n\t%sreturn %s%s\n}\n",
		m.Name, m.Signature(), zeros(m), m.Returns(), "nil")
	if _, err := parser.ParseFile(token.NewFileSet(), "probe.go", src, 0); err != nil {
		t.Errorf("%s %s.%s: the declaration does not parse: %v\n%s", port.Path, port.Name, m.Name, err, src)
	}
	exprs := make([]string, 0, len(m.Params)+len(m.Results))
	for _, p := range slices.Concat(m.Params, m.Results) {
		exprs = append(exprs, p.Type)
	}
	if used := adapterkit.Uses(imports, exprs); !slices.Equal(used, imports) {
		t.Errorf("%s %s.%s: imports %+v, the types use %+v", port.Path, port.Name, m.Name, imports, used)
	}
}

func zeros(m adapterkit.Method) string {
	var b strings.Builder
	for _, z := range m.Zeros {
		fmt.Fprintf(&b, "var %s %s\n\t", z.Name, z.Type)
	}
	return b.String()
}

// kitPorts reads every interface of the kit's non-test Go files; a shape the
// reader refuses is left out, since the reader's own tests hold its refusals.
func kitPorts(t *testing.T, root string) []genkind.Port {
	t.Helper()
	var ports []genkind.Port
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := filepath.ToSlash(strings.TrimPrefix(strings.TrimPrefix(p, root), string(filepath.Separator)))
		if d.IsDir() {
			return skipDir(root, p, rel)
		}
		if path.Ext(rel) != ".go" || strings.HasSuffix(rel, "_test.go") {
			return nil
		}
		content, err := os.ReadFile(p)
		if err != nil {
			return fmt.Errorf("read %s: %w", rel, err)
		}
		found, err := contract.Ports(rel, content)
		if err != nil && !errors.Is(err, genkind.ErrCannotJudge) {
			return fmt.Errorf("read the interfaces of %s: %w", rel, err)
		}
		ports = append(ports, found...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return ports
}

func skipDir(root, p, rel string) error {
	if rel == "" {
		return nil
	}
	if slices.Contains([]string{"testdata", "vendor", "node_modules"}, path.Base(rel)) || strings.HasPrefix(path.Base(rel), ".") {
		return filepath.SkipDir
	}
	if _, err := os.Stat(filepath.Join(p, "go.mod")); err == nil && p != root {
		return filepath.SkipDir
	}
	return nil
}

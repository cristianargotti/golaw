package adapterkit

import (
	"fmt"
	"go/token"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/holes"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
)

// Target is one adapter instance resolved against its port and the layout,
// so both adapter kinds place, name and import an instance the same way.
type Target struct {
	Name     genkind.Name      // the adapter: Area its package, Type its struct
	Port     genkind.Port      // the port as its contract reads
	Contract manifest.Contract // the port's contract path and symbol
	Dir      string            // the adapter's slash directory from the layout
	Package  genkind.Import    // the adapter's package
	PortPkg  genkind.Import    // the port's package
	Suite    genkind.Import    // the port's contract suite package, "<port dir>/<port lower>test"
	Recv     string            // the receiver of the adapter's methods
}

// FromRequest resolves a golaw new request of kind. Every refusal names the
// flag or the command that fixes it, because the request is the agent's and
// a vague usage error would send it guessing.
func FromRequest(in genkind.Input, kind, layout string, req genkind.Request) (Target, error) {
	if err := checkName(kind, req.Name); err != nil {
		return Target{}, err
	}
	name := req.Name.String()
	if req.Port.Area == "" || req.Port.Type == "" {
		return Target{}, fmt.Errorf("%w: %s %s needs --port <area>.<Port>", genkind.ErrRequest, kind, name)
	}
	if entry, ok := in.Manifest.Lookup(kind, name); ok {
		return Target{}, fmt.Errorf("%w: %s %s exists in %s", genkind.ErrExists, kind, name, firstPath(entry))
	}
	port := req.Port.String()
	entry, ok := in.Manifest.Lookup(genkind.KindPort, port)
	if !ok {
		return Target{}, fmt.Errorf("%w: port %s has no manifest entry: run golaw new port %s first",
			genkind.ErrRequest, port, port)
	}
	if len(entry.Contracts) == 0 {
		return Target{}, fmt.Errorf("%w: port %s lists no contract: run golaw gen", genkind.ErrRequest, port)
	}
	return build(in, layout, req.Name, entry.Contracts[0])
}

// FromEntry resolves an adapter entry of the manifest for a regeneration.
func FromEntry(in genkind.Input, layout string, entry manifest.Entry) (Target, error) {
	if len(entry.Contracts) == 0 {
		return Target{}, fmt.Errorf("%w: entry %s lists no contract", genkind.ErrRequest, entry.Name)
	}
	return build(in, layout, genkind.Name{Area: entry.Area(), Type: entry.Type()}, entry.Contracts[0])
}

// Lower is the type name in lower case, the stem of the hole ids and files.
func (t Target) Lower() string { return strings.ToLower(t.Name.Type) }

// Hole is the id of the hole of one port method in this adapter.
func (t Target) Hole(method string) string { return holes.ID(t.Lower(), method) }

// checkName refuses a package or type the layout and the templates could
// not spell as Go.
func checkName(kind string, n genkind.Name) error {
	if !token.IsIdentifier(n.Area) || n.Area != strings.ToLower(n.Area) || strings.Contains(n.Area, "_") {
		return fmt.Errorf("%w: package %q of %s is not a lower case Go package name", genkind.ErrRequest, n.Area, kind)
	}
	if !token.IsIdentifier(n.Type) || !token.IsExported(n.Type) {
		return fmt.Errorf("%w: type %q of %s is not an exported Go name", genkind.ErrRequest, n.Type, kind)
	}
	return nil
}

// firstPath names the file an existing instance lives in, so the refusal
// points at the code a second instance would overwrite.
func firstPath(entry manifest.Entry) string {
	switch {
	case len(entry.Regenerable) > 0:
		return entry.Regenerable[0]
	case len(entry.Owned) > 0:
		return entry.Owned[0]
	}
	return manifest.Path
}

// build reads the port and places the adapter. The reader's error is
// wrapped, never replaced, so a port that needs types keeps its exit code.
func build(in genkind.Input, layout string, name genkind.Name, c manifest.Contract) (Target, error) {
	port, err := in.Contracts.Port(c.Path, c.Symbol)
	if err != nil {
		return Target{}, fmt.Errorf("adapter %s of port %s: %w", name, c.Symbol, err)
	}
	dir := strings.ReplaceAll(layout, "{package}", name.Area)
	portPkg := genkind.Import{Path: in.Module + "/" + port.Dir}
	if port.Package != path.Base(port.Dir) {
		// The import then names the package its clause declares, so the
		// qualified types and the import agree without reading the package.
		portPkg.Name = port.Package
	}
	return Target{
		Name: name, Port: port, Contract: c, Dir: dir,
		Package: genkind.Import{Path: in.Module + "/" + dir},
		PortPkg: portPkg,
		Suite:   genkind.Import{Path: portPkg.Path + "/" + strings.ToLower(port.Name) + "test"},
		Recv:    receiver(name.Type, port),
	}, nil
}

// receiver is the first letter of the type, or the whole lower case name
// when a port parameter already takes that letter, because the receiver
// would shadow it in the generated method.
func receiver(typ string, port genkind.Port) string {
	first, _ := utf8.DecodeRuneInString(typ)
	short := string(unicode.ToLower(first))
	for _, m := range port.Methods {
		for _, p := range m.Params {
			if p.Name == short {
				return strings.ToLower(typ)
			}
		}
	}
	return short
}

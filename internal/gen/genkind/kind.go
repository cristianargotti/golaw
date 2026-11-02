package genkind

import (
	"context"

	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/model"
)

// The kind names of headers and the manifest (M7.3.2): one word each, so a
// header, a manifest entry and a command argument name a kind the same way.
const (
	KindPort        = "port"
	KindAdapterSQL  = "adapter-sql"
	KindAdapterHTTP = "adapter-http"
	KindUsecase     = "usecase"
	KindHandler     = "handler"
	KindWiring      = "wiring"
	KindSupport     = "support"
)

// Name is a kind name as the command takes it: "growth.Ledger" is Area
// "growth" and Type "Ledger"; for an adapter Area is its package. It is split
// because the layout places a kind by area and the templates name it by type.
type Name struct {
	Area string
	Type string
}

// String answers the name as the command and the manifest write it.
func (n Name) String() string { return n.Area + "." + n.Type }

// Request is what golaw new hands a kind, from its flags (M7.3.2); each kind
// reads only the fields its flags fill, so one shape serves every kind.
type Request struct {
	Name     Name
	Port     Name     // adapters: --port growth.Ledger
	Usecase  string   // port: --usecase ReadPlan
	Ports    []string // usecase: --ports Ledger,Clock
	In, Out  string   // usecase: --in ReadPlanInput --out Plan
	API      string   // handler: --api api/openapi.yaml
	Usecases []string // handler: --usecases growth
}

// Input is everything a kind renders from. It holds no file system access,
// the snapshot is the only reader (M7.7), so a kind is pure and a run can be
// planned whole before any byte is written.
type Input struct {
	Module    string               // the module path of go.mod
	Version   string               // the kit version for the header
	Policy    model.GeneratePolicy // the generate block as compiled
	Snapshot  model.Snapshot
	Manifest  manifest.Manifest
	Contracts Contracts
}

// File is one rendered file; the content is formatted Go with LF line endings
// so the bytes compare equal across systems and drift is a byte difference.
type File struct {
	Path    string // slash path from the repository root
	Content []byte
}

// Append is an append only edit of an owned file (M7.4.2 rule 5): owned code
// is the agent's, so a new hole only ever adds its stub and its red test.
type Append struct {
	Path    string
	Decls   [][]byte // each a complete top level declaration, formatted
	Imports []Import
	Holes   []string // the hole ids the declarations open
}

// Binding is one composition root line an entry contributes, collected from
// every entry so the wiring kind renders the root without reading the kinds.
type Binding struct {
	Kind   string     // the producing kind
	Area   string     // the use case area the binding serves
	Port   string     // adapters: the port type name the value is bound to in Deps
	Import Import     // the package that holds the constructor
	Call   string     // the constructor call over the infra value: "postgres.NewPlans(infra.DB)"
	Needs  []InfraUse // the infra fields the call reads
}

// InfraUse is one field of the infra value a binding reads, so the wiring
// declares each field once with its type and import.
type InfraUse struct {
	Field  string // "DB"
	Type   string // "*sql.DB"
	Import Import // the package of the type, "database/sql"
}

// Decision is a contract change the engine will not resolve itself
// (M7.9.4): owned code left without its contract is the agent's to delete or
// keep, so the run reports it in one English line instead of rewriting it.
type Decision struct {
	Path string
	Line int
	Text string
}

// Output is what one kind instance asks the engine to write. Regenerable and
// owned files are split because only the first is ever rewritten.
type Output struct {
	Entry       manifest.Entry
	Regenerable []File   // written whenever their bytes differ, or when absent
	Owned       []File   // written only when the path does not exist yet
	Appends     []Append // stubs for owned files that exist
	Bindings    []Binding
	Decisions   []Decision
	Needs       []Need // other kinds this run must also produce (A56)
}

// Need is another kind instance a kind requires in the same run: a port needs
// its consuming use case (A56), so one command leaves no dangling port.
type Need struct {
	Kind    string
	Request Request
}

// Kind is one generator kind (M7.3.2). New renders a new instance from a
// request; Regenerate renders the regenerable files of an existing entry and
// the appends its owned files lack. Both are pure over the Input.
type Kind interface {
	// Name answers a Kind constant.
	Name() string
	// Template answers the template version this kit embeds.
	Template() int
	New(ctx context.Context, in Input, req Request) (Output, error)
	Regenerate(ctx context.Context, in Input, entry manifest.Entry) (Output, error)
}

// Wirer is the wiring kind's extra entry: it renders the composition root of
// one main package from every binding the other entries contribute.
type Wirer interface { //nolint:iface // the wiring kind implements it and the engine consumes it; the contract package itself never does
	Kind
	Wire(ctx context.Context, in Input, main string, bindings []Binding) (Output, error)
}

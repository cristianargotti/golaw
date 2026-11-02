package genkind

// Import is one import of a contract file or of a generated declaration;
// Name stays empty when the import names no package explicitly, because
// the renderer then lets the path's last element name it.
type Import struct {
	Name string
	Path string
}

// Param is one parameter or result as the contract writes it. The type stays
// the source expression ("context.Context", "growth.AccountID", "[]Plan")
// because the contract reader parses without types (M7.3.1) and a template
// writes it back as it was.
type Param struct {
	Name string // empty for an unnamed parameter or result
	Type string
}

// Method is one method of a port interface, with its doc and its line so a
// hole and a decision point at the contract line that caused them.
type Method struct {
	Name    string
	Doc     string // the doc comment text, without the slashes
	Params  []Param
	Results []Param
	Line    int // 1-based line of the method in the contract
}

// Port is one interface a port, adapter or use case is generated from. It
// carries the file's package, path and imports because the generated code
// must name the contract's types the way the contract file imports them.
type Port struct {
	Name    string   // the interface name
	Package string   // the package clause of the contract file
	Path    string   // the slash path of the contract file
	Dir     string   // the slash directory of the contract file
	Doc     string   // the doc comment of the interface
	Methods []Method // in source order
	Imports []Import // the imports of the contract file
	Line    int
}

// Operation is one operation of the oapi-codegen strict server, the unit a
// handler fills, read from the generated server file and the document.
type Operation struct {
	Name   string // the StrictServerInterface method name: "GetPlan"
	Tag    string // the first tag of the operation, "default" when none
	Method string // the HTTP method, upper case
	Route  string // the path template as the document writes it: "/plans/{id}"
	Line   int    // the line of the method in the oapi-codegen server file
}

// Query is one method of the sqlc Queries type, the unit a SQL adapter calls.
type Query struct {
	Name    string  // "ReadPlan"
	Params  []Param // after ctx
	Results []Param
	File    string // the sqlcgen file that declares it: "plans.sql.go"
}

// Contracts reads the contracts of a run through the run's snapshot. It is an
// interface so the engine's reader implements it and a kind test passes fixed
// answers, which keeps every kind free of parsing and of the file system.
type Contracts interface {
	Port(path, symbol string) (Port, error)
	// Operations reads the oapi-codegen server file and the document.
	Operations(server, openapi string) ([]Operation, error)
	// Queries reads the sqlcgen directory.
	Queries(dir string) ([]Query, error)
}

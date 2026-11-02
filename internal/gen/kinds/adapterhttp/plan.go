package adapterhttp

import (
	"fmt"
	"path"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/kinds/adapterkit"
)

// data is what every template of the kind reads; each name is settled here
// so a template only joins strings and never decides.
type data struct {
	Package        string // the adapter's package name, "planapi"
	Type           string // the adapter's type, "Client"
	Lower          string // the type in lower case, the stem of its files
	Recv           string // the receiver of the methods
	Port           string // the qualified port, "growth.Ledger"
	PortType       string // the port's type name, "Ledger"
	PortPkg        string // the port's package name, "growth"
	Suite          string // the contract suite's package name, "ledgertest"
	Seed           string // the id of the seed hole, "seed.client"
	SeedConstraint string
	Methods        []method
}

// method is one port method with the constraint its hole carries.
type method struct {
	adapterkit.Method

	Constraint string
}

// stub is what a per method template reads: the adapter and the method.
type stub struct {
	D data
	M method
}

// httpPlan is one adapter instance resolved and shaped, rendered as new by
// fresh and as a regeneration by again.
type httpPlan struct {
	target  adapterkit.Target
	data    data
	pkg     adapterkit.Package
	imports []genkind.Import // the imports the port's method types need
}

// plan shapes the port's methods and reads what the adapter's directory
// already declares, so a run appends only what owned code lacks.
func plan(in genkind.Input, t adapterkit.Target) (httpPlan, error) {
	shaped, imports, err := adapterkit.Methods(t)
	if err != nil {
		return httpPlan{}, fmt.Errorf(wrapped, err)
	}
	pkg, err := adapterkit.Scan(in.Snapshot, t.Dir, t.Name.Type)
	if err != nil {
		return httpPlan{}, fmt.Errorf(wrapped, err)
	}
	sentinels, err := adapterkit.Sentinels(in.Snapshot, t.Port, "NotFound")
	if err != nil {
		return httpPlan{}, fmt.Errorf(wrapped, err)
	}
	sentinel := adapterkit.Sentence(sentinels, t.Port.Dir)
	methods := make([]method, 0, len(shaped))
	for i := range shaped {
		methods = append(methods, method{Method: shaped[i], Constraint: constraint(shaped[i], t.Recv, sentinel)})
	}
	pkgName, suite := t.Name.Area, path.Base(t.Suite.Path)
	d := data{
		Package: pkgName, Type: t.Name.Type, Lower: t.Lower(), Recv: t.Recv,
		Port: t.Port.Package + "." + t.Port.Name, PortType: t.Port.Name, PortPkg: t.Port.Package,
		Suite: suite, Seed: "seed." + t.Lower(),
		SeedConstraint: "serve a tlstest.Routes with tlstest.Serve(t, routes) and answer " + suite +
			".Subject{Port: " + pkgName + ".New" + t.Name.Type + "(" + pkgName + "." + t.Name.Type +
			"Config{BaseURL: server.URL, Client: server.Client()}), Seed: add the routes each case's Args and Want need}",
		Methods: methods,
	}
	return httpPlan{target: t, data: d, pkg: pkg, imports: imports}, nil
}

// constraint tells the agent the one way to fill a method: a reading
// method decodes a body, an error only method sends and gates the status,
// and both map the not found answer to the port's sentinel.
func constraint(m adapterkit.Method, recv, sentinel string) string {
	ctx := m.Params[0].Name
	start := "build the URL from " + recv + ".base and the parameters; "
	end := "; wrap every error with gensupport.Prefix(" + m.Action + ")"
	if len(m.Zeros) > 0 {
		return start + "read it with gensupport.Get(" + ctx + ", " + recv + ".client, url); map http.StatusNotFound to " +
			sentinel + "; gate the status with gensupport.Status and decode the body with gensupport.Decode" + end
	}
	return start + "send it with gensupport.Send(" + ctx + ", " + recv + ".client, method, url, body); map http.StatusNotFound to " +
		sentinel + "; gate the status with gensupport.Status" + end
}

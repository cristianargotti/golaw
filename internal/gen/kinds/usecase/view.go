package usecase

import (
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/holes"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
)

// view is what the use case templates render from: every name and text is
// decided here, so a template only places strings.
type view struct {
	Marker, Package string
	Usecase         useView
	Usecases        []useView
	Ports           []portView
}

// useView is one use case as its files and the area files write it.
type useView struct {
	Type, Action, Text, In, Out, Hole, Constraint, PortList, Ready string
	DeclareIn, DeclareOut                                          bool
}

// portView is one port of the area's Deps with its test package.
type portView struct {
	Name, Package string
}

// newView renders e over the package and the area's use cases in all,
// sorted by name, so every entry of an area writes the area files alike.
func newView(pkg areaPackage, e manifest.Entry, all []manifest.Entry) view {
	v := view{Marker: holes.Marker, Package: pkg.Name, Usecase: useOf(pkg, e)}
	var ports []string
	for i := range all {
		v.Usecases = append(v.Usecases, useOf(pkg, all[i]))
		ports = append(ports, all[i].Ports...)
	}
	slices.Sort(ports)
	for _, port := range slices.Compact(ports) {
		v.Ports = append(v.Ports, portView{Name: port, Package: strings.ToLower(port) + "test"})
	}
	return v
}

// useOf names the action, the hole and the guard of one use case. The
// constraint tells the agent the laws the filled Execute must keep, since
// the hole is the only place it writes.
func useOf(pkg areaPackage, e manifest.Entry) useView {
	typ := e.Type()
	u := useView{
		Type:     typ,
		Action:   "action" + typ,
		Text:     action(e.Area(), typ),
		In:       e.In,
		Out:      e.Out,
		Hole:     holes.ID(e.Area(), typ),
		PortList: portList(e.Ports),
		Ready:    "u.deps." + strings.Join(e.Ports, ", u.deps."),
	}
	u.Constraint = "implement " + typ + " over u.deps." + strings.Join(e.Ports, " and u.deps.") +
		"; wrap every port error with gensupport.Prefix(" + u.Action +
		"); keep Execute within 40 lines and cognitive complexity 15 by moving steps into unexported functions of this file"
	u.DeclareIn = !pkg.Types[e.In]
	u.DeclareOut = !pkg.Types[e.Out] && e.Out != e.In
	return u
}

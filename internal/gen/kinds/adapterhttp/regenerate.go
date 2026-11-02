package adapterhttp

import (
	"errors"
	"fmt"
	"io/fs"
	"slices"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/holes"
	"github.com/cristianargotti/golaw/internal/gen/kinds/adapterkit"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/model"
)

// goneText is the decision for an owned file a regeneration would append
// to but cannot find: owned code is the agent's, so it is never rewritten.
const goneText = "%s is gone: restore it, golaw never rewrites owned code"

// again regenerates an existing instance: the regenerable files, one append
// of stubs and one of red tests for the port's new methods, and a decision
// per method owned code keeps without its contract.
func (p httpPlan) again(in genkind.Input, entry manifest.Entry) (genkind.Output, error) {
	tmpl, err := templates()
	if err != nil {
		return genkind.Output{}, err
	}
	regen, err := p.regenerable(in, tmpl)
	if err != nil {
		return genkind.Output{}, err
	}
	shaped := make([]adapterkit.Method, 0, len(p.data.Methods))
	for i := range p.data.Methods {
		shaped = append(shaped, p.data.Methods[i].Method)
	}
	missing, decisions := adapterkit.Reconcile(p.target, shaped, p.pkg, entry.Owned)
	var methods []method
	for i := range p.data.Methods {
		name := p.data.Methods[i].Name
		if slices.ContainsFunc(missing, func(x adapterkit.Method) bool { return x.Name == name }) {
			methods = append(methods, p.data.Methods[i])
		}
	}
	appends, gone, err := p.appends(in, tmpl, methods)
	if err != nil {
		return genkind.Output{}, err
	}
	next := p.entry(regen, entry.Owned)
	return genkind.Output{
		Entry: next, Regenerable: regen, Appends: appends,
		Bindings: p.bindings(in, next), Decisions: append(gone, decisions...),
	}, nil
}

// appends answers the stubs and the red tests of the missing methods, or a
// decision for an owned file that is gone; a red test owned code already
// declares is left out, since an append never redeclares a name.
func (p httpPlan) appends(in genkind.Input, tmpl fs.FS, methods []method) ([]genkind.Append, []genkind.Decision, error) {
	if len(methods) == 0 {
		return nil, nil, nil
	}
	f := p.files()
	var out []genkind.Append
	var gone []genkind.Decision
	if _, ok, err := present(in.Snapshot, f.main); err != nil {
		return nil, nil, err
	} else if !ok {
		gone = append(gone, goneDecision(f.main))
	} else {
		stubs, stubErr := p.stubs(in, tmpl, methods)
		if stubErr != nil {
			return nil, nil, stubErr
		}
		out = append(out, stubs)
	}
	test, ok, err := present(in.Snapshot, f.test)
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return out, append(gone, goneDecision(f.test)), nil
	}
	names, err := holes.Names(test)
	if err != nil {
		return nil, nil, fmt.Errorf(wrapped, err)
	}
	methods = slices.DeleteFunc(slices.Clone(methods), func(m method) bool { return slices.Contains(names, m.Test(p.data.Type)) })
	if len(methods) == 0 {
		return out, gone, nil
	}
	reds, err := p.reds(in, tmpl, methods)
	if err != nil {
		return nil, nil, err
	}
	return append(out, reds), gone, nil
}

func goneDecision(file string) genkind.Decision {
	return genkind.Decision{Path: file, Line: 1, Text: fmt.Sprintf(goneText, file)}
}

// present reads an owned file, answering false for one the snapshot lacks
// and an error only for a read that failed otherwise.
func present(s model.Snapshot, file string) ([]byte, bool, error) {
	content, err := s.Read(file)
	if errors.Is(err, model.ErrNotInSnapshot) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("adapter-http: read %s: %w", file, err)
	}
	return content, true, nil
}

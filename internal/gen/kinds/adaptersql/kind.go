package adaptersql

import (
	"context"
	"fmt"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/kinds/adapterkit"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
)

// wrapped prefixes every error of the kind with the adapter it was for.
const wrapped = "adapter-sql %s: %w"

// templateVersion is the version of templates/gen/adapter-sql this kit embeds.
const templateVersion = 1

type kind struct{}

// Kind answers the adapter-sql kind: a store over the sqlc queries pinned in
// the tools module, whose port methods and row mappings are holes the agent
// fills while the struct, the constructor and the suite stay regenerable.
func Kind() genkind.Kind { return kind{} }

func (kind) Name() string { return genkind.KindAdapterSQL }

func (kind) Template() int { return templateVersion }

// New renders a new adapter of the requested port.
func (kind) New(_ context.Context, in genkind.Input, req genkind.Request) (genkind.Output, error) {
	target, err := adapterkit.FromRequest(in, genkind.KindAdapterSQL, in.Policy.Layout.AdapterSQL, req)
	if err != nil {
		return genkind.Output{}, fmt.Errorf(wrapped, req.Name, err)
	}
	p, err := plan(in, target)
	if err != nil {
		return genkind.Output{}, err
	}
	return p.fresh(in)
}

// Regenerate renders the regenerable files of an existing adapter and the
// appends its owned files lack.
func (kind) Regenerate(_ context.Context, in genkind.Input, entry manifest.Entry) (genkind.Output, error) {
	target, err := adapterkit.FromEntry(in, in.Policy.Layout.AdapterSQL, entry)
	if err != nil {
		return genkind.Output{}, fmt.Errorf(wrapped, entry.Name, err)
	}
	p, err := plan(in, target)
	if err != nil {
		return genkind.Output{}, err
	}
	return p.again(in, entry)
}

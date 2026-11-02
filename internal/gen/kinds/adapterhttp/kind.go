package adapterhttp

import (
	"context"
	"fmt"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/kinds/adapterkit"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
)

// wrapped prefixes every error of the kind, so a run that plans several
// kinds names the one that refused.
const wrapped = "adapter-http: %w"

// templateVersion is the template tree this kit embeds for the kind.
const templateVersion = 1

type kind struct{}

// Kind answers the adapter-http kind: a client of an external API that
// implements one port over the bounded client of the support package.
func Kind() genkind.Kind { return kind{} }

func (kind) Name() string { return genkind.KindAdapterHTTP }

func (kind) Template() int { return templateVersion }

// New renders a new adapter of the requested port. The context is read
// only for a cancellation the engine asked for before the run started,
// because the kind is pure over its input.
func (kind) New(ctx context.Context, in genkind.Input, req genkind.Request) (genkind.Output, error) {
	if err := ctx.Err(); err != nil {
		return genkind.Output{}, fmt.Errorf(wrapped, err)
	}
	target, err := adapterkit.FromRequest(in, genkind.KindAdapterHTTP, in.Policy.Layout.AdapterHTTP, req)
	if err != nil {
		return genkind.Output{}, fmt.Errorf(wrapped, err)
	}
	p, err := plan(in, target)
	if err != nil {
		return genkind.Output{}, err
	}
	return p.fresh(in)
}

// Regenerate renders the regenerable files of an existing adapter again and
// the appends its owned files lack for the port's new methods.
func (kind) Regenerate(ctx context.Context, in genkind.Input, entry manifest.Entry) (genkind.Output, error) {
	if err := ctx.Err(); err != nil {
		return genkind.Output{}, fmt.Errorf(wrapped, err)
	}
	target, err := adapterkit.FromEntry(in, in.Policy.Layout.AdapterHTTP, entry)
	if err != nil {
		return genkind.Output{}, fmt.Errorf(wrapped, err)
	}
	p, err := plan(in, target)
	if err != nil {
		return genkind.Output{}, err
	}
	return p.again(in, entry)
}

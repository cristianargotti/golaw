// Package engine holds the validated immutable registry of checks, the
// applicability rules, the dispatch over a snapshot and the final decision
// the hooks and the scan consume. It imports only model, so a check family
// never depends on it and the proof wraps registry values freely.
package engine

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// Builder collects checks before Build validates and freezes them.
type Builder struct {
	checks map[string]model.Check
}

// New starts an empty builder.
func New() *Builder {
	return &Builder{checks: make(map[string]model.Check)}
}

// Register adds one check. A nil check, an invalid spec or a repeated id is
// refused with ErrInvalidSpec or ErrDuplicateCheck.
func (b *Builder) Register(c model.Check) error {
	if c == nil {
		return fmt.Errorf("%w: nil check", model.ErrInvalidSpec)
	}
	spec := c.Spec()
	if err := spec.Validate(); err != nil {
		return fmt.Errorf("register: %w", err)
	}
	if _, exists := b.checks[spec.ID]; exists {
		return fmt.Errorf("%w: %s", model.ErrDuplicateCheck, spec.ID)
	}
	b.checks[spec.ID] = c
	return nil
}

// BuildOption tunes Build.
type BuildOption func(*buildConfig)

type buildConfig struct {
	proofs    map[string]bool
	withProof bool
}

// WithProofs makes Build refuse a check whose Proof is not among names,
// which the proof test passes from the manifest so a check without a proof
// pair never enters the registry it verifies.
func WithProofs(names []string) BuildOption {
	return func(cfg *buildConfig) {
		cfg.withProof = true
		cfg.proofs = make(map[string]bool, len(names))
		for _, name := range names {
			cfg.proofs[name] = true
		}
	}
}

// Build freezes the registered checks in id order.
func (b *Builder) Build(opts ...BuildOption) (*Registry, error) {
	var cfg buildConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	ids := slices.Sorted(maps.Keys(b.checks))
	reg := &Registry{checks: make([]model.Check, 0, len(ids)), index: make(map[string]int, len(ids))}
	for i, id := range ids {
		c := b.checks[id]
		if proof := c.Spec().Proof; cfg.withProof && !cfg.proofs[proof] {
			return nil, fmt.Errorf("%w: %s names proof %q", model.ErrUnknownProof, id, proof)
		}
		reg.checks = append(reg.checks, c)
		reg.index[id] = i
	}
	return reg, nil
}

// Registry is the frozen, id-ordered set of checks a run dispatches.
type Registry struct {
	checks []model.Check
	index  map[string]int
}

// Checks returns a copy of the checks in id order.
func (r *Registry) Checks() []model.Check {
	return slices.Clone(r.checks)
}

// IDs returns the check ids in order.
func (r *Registry) IDs() []string {
	ids := make([]string, 0, len(r.checks))
	for _, c := range r.checks {
		ids = append(ids, c.Spec().ID)
	}
	return ids
}

// Lookup finds a check by id.
func (r *Registry) Lookup(id string) (model.Check, bool) {
	i, ok := r.index[id]
	if !ok {
		return nil, false
	}
	return r.checks[i], true
}

// Len is the number of checks.
func (r *Registry) Len() int { return len(r.checks) }

// String lists the ids, for errors and logs.
func (r *Registry) String() string { return strings.Join(r.IDs(), ",") }

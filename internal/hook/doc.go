// Package hook is the policy core of the agent adapters: it bounds the
// input, establishes the enrollment and the policy in force, decodes an
// agent event into the normalized model event through the adapter of the
// agent, evaluates it through the engine over the overlay snapshot and
// encodes the answer, with the state of A15 kept outside the checkout.
package hook

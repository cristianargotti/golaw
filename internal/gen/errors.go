package gen

import "errors"

// The errors of a run, matched with errors.Is, so the commands map each one
// to its exit code without reading the entry the engine wraps around it.
var (
	// ErrUnknownKind is a kind name the kit does not hold: a typo in a
	// command or a manifest entry of a newer kit, never something to guess.
	ErrUnknownKind = errors.New("unknown generator kind")
	// ErrUnknownTemplate is a manifest entry whose template version this kit
	// does not embed (M7.6.1): rendering another version would rewrite the
	// files in a shape the entry never asked for.
	ErrUnknownTemplate = errors.New("template version not embedded: run golaw gen --upgrade")
)

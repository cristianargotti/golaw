package gen

import (
	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/holes"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
)

// WriteMode says how apply.Files may land a write, because regenerable code
// is the kit's to replace while owned code is the agent's and is only ever
// created or appended to.
type WriteMode int

const (
	// WriteRegenerable is replaced whenever its bytes differ.
	WriteRegenerable WriteMode = iota + 1
	// WriteOwned is created only and never replaces an existing path.
	WriteOwned
	// WriteAppend is an owned file with stubs appended (M7.4.2 rule 5).
	WriteAppend
	// WriteManifest is .golaw/gen.yml, written by golaw new and golaw gen only.
	WriteManifest
	// WriteDelete removes a regenerable file of an entry the person removed
	// by deleting its owned code; it lands only over the bytes the run read,
	// and owned code is never deleted (A40).
	WriteDelete
)

// Write is one file a run asks apply.Files to land. Base binds it to the
// bytes the plan read, so a concurrent edit is skipped instead of lost.
type Write struct {
	Path    string
	Content []byte
	Mode    WriteMode
	Base    string // source.ContentHash of the bytes read at Path, empty when absent
}

// Drift is one difference golaw gen --check reports, with the command that
// fixes it, so a finding always names its remedy.
type Drift struct {
	Path  string
	Cause string
	Fix   string
}

// The causes of a drift, one English phrase each, so the check output and
// the generate-check rung read the same words.
const (
	DriftDiffers      = "differs from its rendering"
	DriftMissing      = "regenerable file missing"
	DriftContractGone = "contract of the manifest entry is gone"
	DriftThirdParty   = "differs from what its pinned generator writes"
	// DriftStub is an owned file that lacks a stub its contract asks for:
	// golaw gen appends it, so the check names the same command.
	DriftStub = "lacks the stubs of its contract"
	// DriftRemoved is a regenerable file of an entry whose owned files the
	// person deleted: golaw gen removes the entry, so the check names it.
	DriftRemoved = "belongs to an entry whose owned files are gone"
)

// FixGen is the command that settles every drift but a gone contract.
const FixGen = "golaw gen"

// MainByHand is the Main of a repository whose composition roots are
// written by hand. It is not a Go name, so no main package is ever called
// that and it never collides with a real one.
const MainByHand = "-"

// Input is a kind input plus what only the engine reads: the kinds the kit
// holds, the contracts an event changed and the main package of the wiring.
type Input struct {
	genkind.Input

	Kinds   []genkind.Kind // every kind of the kit in production
	Changed []string       // contract paths an event changed; empty means every entry
	Main    string         // the cmd main the wiring renders, empty when the layout is root
}

// Result is the plan of one run: what to write, what drifted, the holes the
// agent fills and the decisions it takes, so callers report without reading
// any file again.
type Result struct {
	Writes    []Write
	Drift     []Drift
	Open      []holes.Hole // the open holes of the owned files, sorted by path and line
	Opened    []holes.Hole
	Closed    []holes.Hole
	Decisions []genkind.Decision
	Entries   []manifest.Entry // the entries the run touched
	// Removed are the entries whose owned files the person deleted, which
	// asks for the entry to go; the run deletes their regenerable files only.
	Removed  []manifest.Entry
	Manifest manifest.Manifest
}

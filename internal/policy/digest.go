package policy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"
)

// canonical is the digest form of a file: the floors and identities with
// every list sorted and every absent block explicit, so two files that
// mean the same policy hash the same whatever their layout.
type canonical struct {
	Schema       int                       `json:"golaw"`
	Kit          string                    `json:"kit"`
	Profile      *ProfileRef               `json:"profile"`
	Language     *string                   `json:"language"`
	Capabilities []string                  `json:"capabilities"`
	Layout       *Layout                   `json:"layout"`
	Sizes        *Sizes                    `json:"sizes"`
	Coverage     *Coverage                 `json:"coverage"`
	Terms        *Terms                    `json:"terms"`
	Lint         []string                  `json:"lint"`
	Shell        *Shell                    `json:"shell"`
	Options      map[string]map[string]any `json:"options"`
	Waivers      []Waiver                  `json:"waivers"`
	Exceptions   []ExceptionRule           `json:"exceptions"`
}

// Digest is the lower case hex sha256 over the canonical form of a file:
// the schema, the kit pin, the profile, the language, the capabilities,
// the layout, every floor block, the per-check options and the waiver and
// exception sets, each list sorted. The ladder and the managed hashes are
// left out: they are local convenience and drift bookkeeping, not policy,
// so toggling a rung never asks for a new enrollment.
func Digest(f File) string {
	c := canonical{
		Schema:       f.Schema,
		Kit:          f.Kit.Version,
		Profile:      f.Profile,
		Language:     f.Language,
		Capabilities: sortedUnique(f.Capabilities),
		Layout:       f.Layout,
		Sizes:        f.Checks.Sizes,
		Coverage:     f.Checks.Coverage.canonical(),
		Terms:        f.Checks.Terms.canonical(),
		Lint:         []string{},
		Shell:        f.Checks.Shell,
		Options:      f.Checks.Options,
		Waivers:      slices.Clone(f.Waivers),
		Exceptions:   slices.Clone(f.Exceptions),
	}
	if f.Checks.Lint != nil {
		c.Lint = f.Checks.Lint.entries()
	}
	if c.Options == nil {
		c.Options = map[string]map[string]any{}
	}
	slices.SortFunc(c.Waivers, func(a, b Waiver) int { return strings.Compare(a.ID+a.Until+a.Record, b.ID+b.Until+b.Record) })
	slices.SortFunc(c.Exceptions, func(a, b ExceptionRule) int {
		return strings.Compare(a.Check+"\x00"+a.Path+"\x00"+a.Record+a.Until, b.Check+"\x00"+b.Path+"\x00"+b.Record+b.Until)
	})
	encoded, err := json.Marshal(c)
	if err != nil {
		// Every field is a plain value or a map of plain values; the encoder
		// cannot refuse them, so an error here is a programming mistake.
		panic(err)
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

// canonical answers the coverage block with its exemptions sorted.
func (c *Coverage) canonical() *Coverage {
	if c == nil {
		return nil
	}
	out := *c
	out.FileExemptions = sortedUnique(c.FileExemptions)
	return &out
}

// canonical answers the terms block with both lists sorted.
func (t *Terms) canonical() *Terms {
	if t == nil {
		return nil
	}
	return &Terms{Tools: sortedUnique(t.Tools), AllowPaths: sortedUnique(t.AllowPaths)}
}

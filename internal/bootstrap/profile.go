package bootstrap

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

// profileLine is the one flow line init writes for the pin, at the start
// of a line; the braces are what SetProfile rewrites.
var profileLine = regexp.MustCompile(`(?m)^profile:[ \t]*\{[^}\n]*\}`)

// kitLine is the kit pin line the profile line follows in a fresh policy.
var kitLine = regexp.MustCompile(`(?m)^kit:[^\n]*\n`)

// SetProfile answers a reviewed policy with its profile line set to ref:
// the flow line init writes replaced in place, or inserted after the kit
// line when the file has none (at the end of a file without a kit line);
// the bytes come back unchanged when the file already pins ref. Any other
// form is left to the person with the line to write, as GrowCapabilities
// does, and the tightening check judges the pin itself (A19).
func SetProfile(current []byte, ref policy.ProfileRef) ([]byte, error) {
	config, err := policy.Decode(current)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", PolicyFile, err)
	}
	if config.Profile != nil && *config.Profile == ref {
		return current, nil
	}
	line := renderProfileLine(ref)
	var proposed []byte
	switch found := profileLine.FindAllIndex(current, -1); {
	case len(found) == 1:
		at := found[0]
		proposed = slices.Concat(current[:at[0]], []byte(line), current[at[1]:])
	case len(found) == 0 && config.Profile == nil:
		proposed = insertProfileLine(current, line)
	}
	if proposed != nil {
		if config, err = policy.Decode(proposed); err == nil && config.Profile != nil && *config.Profile == ref {
			return proposed, nil
		}
	}
	return nil, fmt.Errorf("%w: %s names its profile in a form init does not edit; write %s in %s by hand",
		model.ErrUsage, PolicyFile, line, PolicyFile)
}

// renderProfileLine is the flow line of a pin, the one form init writes
// and SetProfile edits.
func renderProfileLine(ref policy.ProfileRef) string {
	return fmt.Sprintf("profile: { name: %s, source: %s, version: %s, digest: %s }", ref.Name, ref.Source, ref.Version, ref.Digest)
}

// insertProfileLine puts the line after the kit pin, or at the end of a
// file without one, with a newline before it when the file lacks one.
func insertProfileLine(current []byte, line string) []byte {
	if at := kitLine.FindIndex(current); at != nil {
		return slices.Concat(current[:at[1]], []byte(line+"\n"), current[at[1]:])
	}
	if len(current) > 0 && !bytes.HasSuffix(current, []byte("\n")) {
		current = append(slices.Clone(current), '\n')
	}
	return slices.Concat(current, []byte(line+"\n"))
}

package bootstrap

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

// capabilityLine is the one flow line init writes for the capabilities, at
// the start of a line; the bracket is what GrowCapabilities rewrites.
var capabilityLine = regexp.MustCompile(`(?m)^capabilities:[ \t]*\[[^\]\n]*\]`)

// GrowCapabilities answers a reviewed policy with the named capabilities
// added to its capabilities line, sorted and unique, and every capability
// it holds kept, because capabilities only grow (CheckTightening); the
// bytes come back unchanged when the names add nothing. It edits only the
// flow line init writes, or appends one when the file has no capabilities
// key; any other form is left to the person, with the line to write.
func GrowCapabilities(current []byte, names []string) ([]byte, error) {
	if len(names) == 0 {
		return current, nil
	}
	config, err := policy.Decode(current)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", PolicyFile, err)
	}
	grown := slices.Clone(config.Capabilities)
	for _, name := range names {
		if !slices.Contains(model.Capabilities(), name) {
			return nil, fmt.Errorf("%w: unknown capability %q", model.ErrUsage, name)
		}
		if !slices.Contains(grown, name) {
			grown = append(grown, name)
		}
	}
	if len(grown) == len(config.Capabilities) {
		return current, nil
	}
	slices.Sort(grown)
	if err := policy.CheckCapabilities(grown); err != nil {
		return nil, fmt.Errorf("%w: %w", model.ErrUsage, err)
	}
	return writeCapabilities(current, config.Capabilities, grown)
}

// writeCapabilities puts the grown set in the one capabilities line, or in
// a line of its own at the end of a file without the key.
func writeCapabilities(current []byte, held, grown []string) ([]byte, error) {
	line := "capabilities: [" + strings.Join(grown, ", ") + "]"
	switch found := capabilityLine.FindAllIndex(current, -1); {
	case len(found) == 1:
		at := found[0]
		return slices.Concat(current[:at[0]], []byte(line), current[at[1]:]), nil
	case len(found) == 0 && len(held) == 0 && !bytes.Contains(current, []byte("\ncapabilities:")) && !bytes.HasPrefix(current, []byte("capabilities:")):
		if len(current) > 0 && !bytes.HasSuffix(current, []byte("\n")) {
			current = append(slices.Clone(current), '\n')
		}
		proposed := slices.Concat(current, []byte(line+"\n"))
		config, err := policy.Decode(proposed)
		if err != nil || !slices.Equal(config.Capabilities, grown) {
			break
		}
		return proposed, nil
	}
	return nil, fmt.Errorf("%w: %s lists its capabilities in a form init does not edit; write %s in %s by hand",
		model.ErrUsage, PolicyFile, line, PolicyFile)
}

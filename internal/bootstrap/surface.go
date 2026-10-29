package bootstrap

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

// surfaceLine is the one flow line init writes for the surface floor, at
// the start of a line; the braces are what SetSurfaceRules rewrites.
var surfaceLine = regexp.MustCompile(`(?m)^surface:[ \t]*\{[^}\n]*\}`)

// SetSurfaceRules answers a policy with its surface floor set to rules: the
// flow line init writes replaced in place, or appended when the file names
// no surface block; the bytes come back unchanged when the file already
// floors at rules. Any other form is left to the person with the line to
// write, as SetProfile does: a flow line that nests a mapping does not
// decode once rewritten, so init never drops a key the person wrote.
func SetSurfaceRules(current []byte, rules string) ([]byte, error) {
	if rules != model.SurfaceParity && rules != model.SurfaceStrict {
		return nil, fmt.Errorf("%w: --surface-rules takes parity or strict, not %q", model.ErrUsage, rules)
	}
	config, err := policy.Decode(current)
	if err != nil {
		return nil, fmt.Errorf("read the surface floor of %s: %w", PolicyFile, err)
	}
	if surfaceRules(config.Surface) == rules {
		return current, nil
	}
	line := "surface: { rules: " + rules + " }"
	var proposed []byte
	switch found := surfaceLine.FindAllIndex(current, -1); {
	case len(found) == 1:
		at := found[0]
		proposed = slices.Concat(current[:at[0]], []byte(line), current[at[1]:])
	case len(found) == 0 && config.Surface == nil:
		proposed = appendLine(current, line)
	}
	if proposed != nil {
		if config, err = policy.Decode(proposed); err == nil && surfaceRules(config.Surface) == rules {
			return proposed, nil
		}
	}
	return nil, fmt.Errorf("%w: %s names its surface block in a form init does not edit; write %s in %s by hand",
		model.ErrUsage, PolicyFile, line, PolicyFile)
}

// surfaceRules is the floor a decoded block names, empty when unset.
func surfaceRules(s *policy.Surface) string {
	if s == nil || s.Rules == nil {
		return ""
	}
	return *s.Rules
}

// appendLine puts the line at the end of the file, with a newline before
// it when the file lacks one.
func appendLine(current []byte, line string) []byte {
	if len(current) > 0 && !bytes.HasSuffix(current, []byte("\n")) {
		current = append(slices.Clone(current), '\n')
	}
	return slices.Concat(current, []byte(line+"\n"))
}

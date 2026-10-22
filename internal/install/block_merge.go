package install

import (
	"bytes"
	"fmt"
)

// blockBudget bounds the block an instruction file carries, so the kit
// never takes a large share of the context a host loads every session.
const blockBudget = 1000

// mergeAgentBlock writes the marked block of the kit into current: an
// existing block is replaced in place and every byte around it is kept,
// otherwise block is appended after one separating line break when the
// file does not already end with one, so merging the same block twice
// answers the same bytes and a reinstall never grows the person's file.
func mergeAgentBlock(current, block []byte) ([]byte, error) {
	content, err := blockSource(block)
	if err != nil {
		return nil, err
	}
	start, end, found, err := agentBlockSpan(current)
	if err != nil {
		return nil, err
	}
	if found {
		merged := make([]byte, 0, start+len(content)+len(current)-end)
		merged = append(merged, current[:start]...)
		merged = append(merged, content...)
		return append(merged, current[end:]...), nil
	}
	merged := make([]byte, 0, len(current)+1+len(block))
	merged = append(merged, current...)
	if len(current) > 0 && current[len(current)-1] != '\n' {
		merged = append(merged, '\n')
	}
	return append(merged, block...), nil
}

// blockSource answers the span of a source that is exactly one marked
// block, optionally followed by one line break, within the budget; any
// other text would land outside the markers, where uninstall could
// never find it again.
func blockSource(source []byte) ([]byte, error) {
	if len(source) > blockBudget {
		return nil, fmt.Errorf("agent block: %d bytes over the budget of %d: %w", len(source), blockBudget, ErrSettings)
	}
	start, end, found, err := agentBlockSpan(source)
	if err != nil {
		return nil, err
	}
	rest := source[end:]
	if !found || start != 0 || (len(rest) > 0 && !bytes.Equal(rest, []byte("\n"))) {
		return nil, fmt.Errorf("agent block: the source is not one marked block: %w", ErrSettings)
	}
	return source[:end], nil
}

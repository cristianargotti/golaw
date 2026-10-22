package doctor

import (
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/install"
)

// CaptureRecord is what a hook process recorded of the payloads its
// adapter could not judge yet (A4), for doctor to report: the repository
// it ran for, the hook definition it ran under and, per host event and
// tool, the key names of the last payload, never a value.
type CaptureRecord struct {
	Identity   string         `json:"identity"`
	HookDigest string         `json:"hook_digest"`
	Captures   []hook.Capture `json:"captures"`
	SeenAt     time.Time      `json:"seen_at"`
}

// SaveCapture merges the captures of a record into the record at path:
// one entry per host event and tool, the latest keys kept, in event and
// tool order. A record of another repository or definition, or one that
// does not decode, is replaced, because it is evidence of nothing now.
func SaveCapture(path string, record CaptureRecord) error {
	merged := record
	if data, err := os.ReadFile(path); err == nil { //nolint:gosec // fallback: the command selects the state capture file, and an absent one starts a record
		var previous CaptureRecord
		if json.Unmarshal(data, &previous) == nil && previous.Identity == record.Identity && previous.HookDigest == record.HookDigest {
			merged.Captures = mergeCaptures(previous.Captures, record.Captures)
		}
	}
	data, err := json.Marshal(merged)
	if err != nil {
		return fmt.Errorf("encode capture record: %w", err)
	}
	if err := install.WriteAtomic(path, append(data, '\n')); err != nil {
		return fmt.Errorf("write capture record: %w", err)
	}
	return nil
}

// mergeCaptures keeps the latest capture of each event and tool.
func mergeCaptures(previous, latest []hook.Capture) []hook.Capture {
	out := slices.Clone(latest)
	for _, c := range previous {
		if !slices.ContainsFunc(latest, func(l hook.Capture) bool { return l.Event == c.Event && l.Tool == c.Tool }) {
			out = append(out, c)
		}
	}
	slices.SortFunc(out, func(a, b hook.Capture) int {
		return cmp.Or(cmp.Compare(a.Event, b.Event), cmp.Compare(a.Tool, b.Tool))
	})
	return out
}

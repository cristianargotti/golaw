package doctor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/cristianargotti/golaw/internal/hook"
)

// LoadCaptures reads the captures a hook process recorded for a repository
// under the hook definition installed now (A4): an absent record holds
// none, and so does a record of another repository or definition, which is
// evidence of nothing now; a record that cannot be read is an error the
// report names.
func LoadCaptures(path, identity, hookDigest string) ([]hook.Capture, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the command selects the state capture file
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read the capture record: %w", err)
	}
	var record CaptureRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("decode the capture record: %w", err)
	}
	if record.Identity != identity || record.HookDigest != hookDigest {
		return nil, nil
	}
	return record.Captures, nil
}

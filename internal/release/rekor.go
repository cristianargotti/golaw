package release

import (
	"fmt"
	"time"
)

// entryTime is the time the first log entry of a bundle says the log
// integrated the signature, the time every certificate is judged at.
func entryTime(entries []tlogEntry) (time.Time, error) {
	if len(entries) == 0 {
		return time.Time{}, fmt.Errorf("%w: the bundle carries no transparency log entry", errSignature)
	}
	return time.Unix(int64(entries[0].IntegratedTime), 0).UTC(), nil
}

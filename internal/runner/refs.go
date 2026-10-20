package runner

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
)

// ZeroSHA retains the SHA-1 spelling used by existing hook callers.
const ZeroSHA = "0000000000000000000000000000000000000000"

const (
	sha1HexLength   = 40
	sha256HexLength = 64
	refFieldCount   = 4
)

// RefUpdate preserves the advertised objects, including annotated tags.
type RefUpdate struct {
	LocalRef  string
	LocalSHA  string
	RemoteRef string
	RemoteSHA string
}

// ParseRefUpdates reads the entire hook input so no pushed ref is omitted.
func ParseRefUpdates(r io.Reader) ([]RefUpdate, error) {
	reader := bufio.NewReader(r)
	var updates []RefUpdate
	for line := 1; ; line++ {
		text, readErr := reader.ReadString('\n')
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return nil, fmt.Errorf("read pre-push line %d: %w", line, readErr)
		}
		fields := strings.Fields(text)
		if len(fields) != 0 {
			update, err := parseRefLine(text, fields)
			if err != nil {
				return nil, fmt.Errorf("line %d %q: %w", line, strings.TrimRight(text, "\r\n"), err)
			}
			updates = append(updates, update)
		}
		if errors.Is(readErr, io.EOF) {
			return updates, nil
		}
	}
}

// parseRefLine reads the objects and the remote ref from the end of the
// line, since none of them holds a space, and keeps the text before them as
// the local ref: git writes a source expression as it was typed, so
// HEAD@{1 minute ago} reaches the hook as one ref with its spaces.
func parseRefLine(text string, fields []string) (RefUpdate, error) {
	if len(fields) < refFieldCount {
		return RefUpdate{}, fmt.Errorf("%w: expected at least four fields", ErrRefUpdate)
	}
	tail := fields[len(fields)-refFieldCount+1:]
	local := strings.TrimSpace(text)
	for _, field := range slices.Backward(tail) {
		local = strings.TrimSpace(strings.TrimSuffix(local, field))
	}
	update := RefUpdate{local, tail[0], tail[1], tail[2]}
	if err := validateUpdate(update); err != nil {
		return RefUpdate{}, err
	}
	return update, nil
}

func validateUpdate(update RefUpdate) error {
	if !validObject(update.LocalSHA) || !validObject(update.RemoteSHA) {
		return fmt.Errorf("%w: invalid object pair %q %q", ErrRefUpdate, update.LocalSHA, update.RemoteSHA)
	}
	return nil
}

func validObject(object string) bool {
	if len(object) != sha1HexLength && len(object) != sha256HexLength {
		return false
	}
	_, err := hex.DecodeString(object)
	return err == nil
}

func zeroObject(object string) bool {
	return strings.Trim(object, "0") == ""
}

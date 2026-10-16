package hook

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

const (
	maxIdentityPaths = 65536
	maxIdentityBytes = 64 << 20
	absentIdentity   = "absent"
)

// StateKey prevents one tool invocation from borrowing another's baseline.
func StateKey(repository string, event model.Event) (string, error) {
	if repository == "" || event.Agent == "" ||
		event.SessionID == "" || event.ToolUseID == "" {
		return "", fmt.Errorf("%w: incomplete state identity", model.ErrCannotJudge)
	}
	return Hash([]byte(joinParts([]string{
		repository, event.Agent, event.SessionID, event.AgentID, event.ToolUseID,
	}))), nil
}

// Identities detects modifications even when a file was already dirty.
func Identities(ctx context.Context, snapshot model.Snapshot, paths []string) (map[string]string, error) {
	if paths == nil {
		for _, file := range snapshot.Files() {
			paths = append(paths, file.Path)
		}
	}
	if len(paths) > maxIdentityPaths {
		return nil, fmt.Errorf("%w: content identity path bound exceeded", model.ErrCannotJudge)
	}
	out := make(map[string]string, len(paths))
	total := 0
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("%w: capture: %w", model.ErrDeadline, err)
		}
		identity, size, err := pathIdentity(snapshot, path)
		if err != nil {
			return nil, err
		}
		total += size
		if total > maxIdentityBytes {
			return nil, fmt.Errorf("%w: content identity byte bound exceeded", model.ErrCannotJudge)
		}
		out[path] = identity
	}
	return out, nil
}

func pathIdentity(snapshot model.Snapshot, path string) (string, int, error) {
	if !filepath.IsLocal(filepath.FromSlash(path)) || path == "." {
		return "", 0, fmt.Errorf("%w: invalid snapshot path", model.ErrCannotJudge)
	}
	if snapshot.Class(path) == model.ClassUnknown {
		return absentIdentity, 0, nil
	}
	content, err := snapshot.Read(path)
	if err != nil {
		return "", 0, fmt.Errorf("%w: content identity: %w", model.ErrCannotJudge, err)
	}
	if len(content) > model.MaxReadBytes {
		return "", 0, fmt.Errorf("%w: content identity: %w", model.ErrCannotJudge, model.ErrTooLarge)
	}
	return Hash(content), len(content), nil
}

// MapHash makes serialized identities independent of map iteration order.
func MapHash(values map[string]string) string {
	keys := slices.Sorted(maps.Keys(values))
	parts := make([]string, 0, len(keys)*2)
	for _, key := range keys {
		parts = append(parts, key, values[key])
	}
	return Hash([]byte(joinParts(parts)))
}

// Changed includes additions and deletions as well as edited existing files.
func Changed(before, after map[string]string) []string {
	names := make(map[string]bool, len(before)+len(after))
	for path, identity := range before {
		if after[path] != identity {
			names[path] = true
		}
	}
	for path, identity := range after {
		if before[path] != identity {
			names[path] = true
		}
	}
	return slices.Sorted(maps.Keys(names))
}

func joinParts(parts []string) string {
	encoded := make([]string, len(parts))
	for i, part := range parts {
		encoded[i] = strconv.Itoa(len(part)) + ":" + part
	}
	return strings.Join(encoded, "")
}

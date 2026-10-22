package codex

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// postRequest is the request of a PostToolUse event of either canonical
// tool, after its tool ran: the response kept as the host wrote it, and
// for a patch the files it wrote as the hints the core judges, their size
// measured from what is on disk now. A Bash event keeps its tool name, so
// the core's detection pass judges what the command changed, whatever its
// exit: Codex sends no exit status and no failure event (survey section
// 2.5), so a failure is never inferred from the output text.
func postRequest(event model.Event, command string, response json.RawMessage, base hook.Base) (hook.Request, error) {
	text, err := responseText(response)
	if err != nil {
		return hook.Request{}, err
	}
	post := model.PostResult{Response: text}
	event.Post = &post
	if event.Tool != patchTool {
		return hook.Request{Event: event}, nil
	}
	hints, err := patchHints(event.Cwd, command, base)
	if err != nil {
		return hook.Request{}, err
	}
	return hook.Request{Event: event, Hints: hints}, nil
}

// responseText is the tool_response of a post event: a string as its
// text, and any other JSON value, which the survey leaves free, as its
// compact JSON, its fields never read. An event without the field cannot
// be judged, because Codex always writes it.
func responseText(response json.RawMessage) (string, error) {
	trimmed := bytes.TrimSpace(response)
	if len(trimmed) == 0 {
		return "", fmt.Errorf("%w: the Codex post event has no tool_response", model.ErrCannotJudge)
	}
	if trimmed[0] == '"' {
		var text string
		if err := json.Unmarshal(trimmed, &text); err != nil {
			return "", fmt.Errorf("%w: Codex tool_response: %w", model.ErrCannotJudge, err)
		}
		return text, nil
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, trimmed); err != nil {
		return "", fmt.Errorf("%w: Codex tool_response: %w", model.ErrCannotJudge, err)
	}
	return compact.String(), nil
}

// patchHints are the files a patch that ran wrote, from the root of the
// repository: every hunk path and Move to path, the moved and the deleted
// ones included, sorted and once each. The patch is not applied again,
// since its files already changed, and a path the repository does not hold
// is no file of it to judge.
func patchHints(cwd, command string, base hook.Base) ([]string, error) {
	hunks, err := ParsePatch(command)
	if err != nil {
		return nil, err
	}
	var hints []string
	for _, hunk := range hunks {
		for _, name := range []string{hunk.Path, hunk.To} {
			if name == "" {
				continue
			}
			if path, placeErr := base.Relative(cwd, exposed(cwd, name)); placeErr == nil {
				hints = append(hints, path)
			}
		}
	}
	slices.Sort(hints)
	return slices.Compact(hints), nil
}

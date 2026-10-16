package claude

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

func postRequest(w wire, event model.Event, in toolInput, base hook.Base) (hook.Request, error) {
	response := w.Response
	if event.Post.Failed {
		if event.Tool != bashTool && event.Tool != powerTool {
			return hook.Request{}, fmt.Errorf("%w: unsupported failure event", model.ErrCannotJudge)
		}
		response = w.Error
	}
	if len(response) == 0 || string(response) == "null" {
		return hook.Request{}, fmt.Errorf("%w: post result is absent", model.ErrCannotJudge)
	}
	event.Post.Response = string(response)
	var hints []string
	var err error
	switch event.Tool {
	case writeTool, editTool, notebookTool:
		hints, err = writeHints(w, in, base)
	case bashTool, powerTool:
		hints = shellHints(w, base)
	default:
		err = fmt.Errorf("%w: unsupported post tool", model.ErrCannotJudge)
	}
	if err != nil {
		return hook.Request{}, err
	}
	return hook.Request{Event: event, Hints: hints}, nil
}

// writeHints names the file a post event of a write tool wrote: the path
// its one parse read, file_path or notebook_path as its tool names it.
func writeHints(w wire, in toolInput, base hook.Base) ([]string, error) {
	if in.err != nil {
		return nil, fmt.Errorf("%w: post write input: %w", model.ErrCannotJudge, in.err)
	}
	relative, err := base.Relative(w.Cwd, in.target)
	if err != nil {
		return nil, fmt.Errorf("%w: post write path: %w", model.ErrCannotJudge, err)
	}
	return []string{relative}, nil
}

func shellHints(w wire, base hook.Base) []string {
	var response struct {
		Diff struct {
			Changed []string `json:"changedFiles"`
			Files   []struct {
				Path string `json:"filePath"`
			} `json:"files"`
		} `json:"bashEditDiff"`
	}
	if err := json.Unmarshal(w.Response, &response); err != nil {
		return nil
	}
	paths := response.Diff.Changed
	for _, file := range response.Diff.Files {
		paths = append(paths, file.Path)
	}
	var out []string
	for _, path := range paths {
		relative, err := base.Relative(w.Cwd, path)
		if err == nil {
			out = append(out, relative)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

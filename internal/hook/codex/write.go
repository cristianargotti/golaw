package codex

import (
	"cmp"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// patchTargets are the files a patch writes, for enrollment before any
// file is read: the target is the first hunk path or Move to path, in hunk
// order, that lies under the working directory, else the first hunk's
// path, and the others follow, each as written and once, so a first path
// outside every repository never lends its unenrolled location to the
// files inside one (A3). A text that does not parse exposes nothing, and
// the enrolled decoder names the error.
func patchTargets(cwd, text string) (string, []string) {
	hunks, err := ParsePatch(text)
	if err != nil || len(hunks) == 0 {
		return "", nil
	}
	var paths []string
	for _, hunk := range hunks {
		for _, path := range []string{hunk.Path, hunk.To} {
			if name := exposed(cwd, path); path != "" && !slices.Contains(paths, name) {
				paths = append(paths, name)
			}
		}
	}
	target := paths[0]
	if at := slices.IndexFunc(paths, func(path string) bool { return underCwd(cwd, path) }); at >= 0 {
		target = paths[at]
	}
	return target, slices.DeleteFunc(slices.Clone(paths), func(path string) bool { return path == target })
}

// underCwd reports whether a header path lies under the working directory:
// joined to it when relative and kept when absolute, in the host's
// separators and cleaned, so an absolute header under the working
// directory counts as local.
func underCwd(cwd, path string) bool {
	full := filepath.Clean(filepath.FromSlash(path))
	if !filepath.IsAbs(full) {
		full = filepath.Join(filepath.FromSlash(cwd), full)
	}
	rel, err := filepath.Rel(filepath.Clean(filepath.FromSlash(cwd)), full)
	return err == nil && filepath.IsLocal(rel)
}

// exposed is a header path as the core reads it: as written, except a path
// that starts with a tilde, which Codex joins to its working directory
// while the core would expand it from the home, so it is exposed joined.
func exposed(cwd, path string) string {
	if strings.HasPrefix(strings.TrimSpace(path), "~") {
		return filepath.Join(cwd, path)
	}
	return path
}

// patchPath is the path of a hunk from the root of the repository the
// patch is judged in, through Base.Relative; a path that repository does
// not hold is named as written with the fix the agent can apply, because
// the hook judges one repository per event (Review Focus 2 of the plan).
func patchPath(base hook.Base, cwd, name string) (string, error) {
	path, err := base.Relative(cwd, exposed(cwd, name))
	if err == nil {
		return path, nil
	}
	if !filepath.IsAbs(cwd) || !filepath.IsAbs(base.Root) {
		return "", fmt.Errorf("%w: apply_patch path %s: %w", model.ErrCannotJudge, name, err)
	}
	return "", fmt.Errorf("%w: %s lies outside the repository this patch is judged in: apply it in a separate patch", model.ErrCannotJudge, name)
}

// patchRequest is the request of an apply_patch pre event: one candidate
// per hunk with its bytes. An empty patch is observed as the no-op it is in
// any environment, since ordinary write validation requires a candidate; a
// patch of another environment is refused before any file is read,
// because the mapping of an environment to the local files is unproven
// (survey section 2.3).
func patchRequest(event model.Event, text string, base hook.Base, snapshot model.Snapshot) (hook.Request, error) {
	doc, err := parsePatch(text)
	if err != nil {
		return hook.Request{}, err
	}
	event.Shape = model.ShapeWrite
	switch {
	case len(doc.Hunks) == 0:
		return hook.Request{Event: event, Observe: true}, nil
	case doc.Environment != "":
		return hook.Request{}, environmentRefusal(doc.Environment)
	}
	request, over, err := patchCandidates(event, doc.Hunks, patchAdded(text), base, snapshot)
	if err == nil && over != "" && len(request.Content) > 0 {
		// The core judges a request with a gap by names alone, which would
		// leave the bytes of the other files of the patch unjudged.
		err = fmt.Errorf("%w: %s is over the 8 MiB the hook reads, so the other files of this patch cannot be judged beside it: apply its change in a separate patch", model.ErrCannotJudge, over)
	}
	if err != nil {
		return hook.Request{}, err
	}
	return request, nil
}

// patchCandidates are the candidates of the hunks with their bytes, and the
// first base over the read bound, whose candidate is left to the pipeline
// with a notice, because it cannot be reconstructed within the bound.
func patchCandidates(event model.Event, hunks []Hunk, added [][]string, base hook.Base, snapshot model.Snapshot) (hook.Request, string, error) {
	request := hook.Request{Event: event, Content: map[string][]byte{}}
	over := ""
	for i, lines := range added {
		candidate, content, err := candidateOf(hunks[i], lines, event.Cwd, base, snapshot)
		switch {
		case errors.Is(err, model.ErrTooLarge):
			request.Gaps = append(request.Gaps, hook.OverReadBound(candidate.Path))
			over = cmp.Or(over, candidate.From, candidate.Path)
		case err != nil:
			return hook.Request{}, "", err
		case content != nil:
			request.Content[candidate.Path] = content
		}
		request.Event.Candidates = append(request.Event.Candidates, candidate)
	}
	return request, over, nil
}

// environmentRefusal is the answer to a patch that selects a Codex
// environment, whose files this hook cannot read.
func environmentRefusal(id string) error {
	return fmt.Errorf("%w: the patch selects the Codex environment %q, whose files this hook cannot read: apply it in the local environment, without the Environment ID line", model.ErrCannotJudge, id)
}

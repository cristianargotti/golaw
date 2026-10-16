package hook

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
)

// unreadView is a snapshot in which the paths of a write the hook could
// not hold read as over the read bound, so the name rules judge them and
// no rule reads the bytes the base held there before the write.
type unreadView struct {
	base  model.Snapshot
	files map[string]model.File
}

// overBound is the file of a path whose bytes the hook cannot hold: its
// class as the base knows it, untracked when the base holds none, and a
// size over the read bound.
func overBound(base model.Snapshot, path string) model.File {
	class := base.Class(path)
	if class == model.ClassUnknown {
		class = model.ClassUntracked
	}
	return model.File{Path: path, Class: class, Size: model.MaxReadBytes + 1}
}

func (v unreadView) Files() []model.File {
	return v.fold(v.base.Files(), func(string) bool { return true })
}

func (v unreadView) Select(match func(path string) bool) []model.File {
	return v.fold(model.Select(v.base, match), match)
}

func (v unreadView) Any(match func(path string) bool) bool {
	for path := range v.files {
		if match(path) {
			return true
		}
	}
	return model.Any(v.base, match)
}

func (v unreadView) Read(path string) ([]byte, error) {
	if _, held := v.files[path]; held {
		return nil, fmt.Errorf("%w: %s is a write over the bound", model.ErrTooLarge, path)
	}
	return v.base.Read(path) //nolint:wrapcheck // the base names the path in its error
}

func (v unreadView) Class(path string) model.FileClass {
	if f, held := v.files[path]; held {
		return f.Class
	}
	return v.base.Class(path)
}

// fold puts the files of the view in place of the base's, in path order.
func (v unreadView) fold(files []model.File, match func(string) bool) []model.File {
	out := slices.DeleteFunc(files, func(f model.File) bool { _, held := v.files[f.Path]; return held })
	for path, f := range v.files {
		if match(path) {
			out = append(out, f)
		}
	}
	slices.SortFunc(out, func(a, b model.File) int { return strings.Compare(a.Path, b.Path) })
	return out
}

// judgeUnread runs the rules over files whose bytes the hook cannot hold:
// the name rules judge them and every rule that needs their bytes leaves
// them unread, so a name the tree forbids is answered as any finding and
// anything else is allowed with the notice of the gaps.
func judgeUnread(ctx context.Context, dependencies Dependencies, in model.Input, gaps []Gap, language string) Result {
	dispatched, err := engine.Dispatch(ctx, dependencies.Registry, in)
	if err != nil {
		return failureLanguage(*in.Event, err, language)
	}
	decision := engine.Decide(dispatched)
	if decision.Verdict == engine.VerdictClean {
		return noticed(withUnread(gaps, decision.Unread), language)
	}
	return decisionResult(*in.Event, decision, language)
}

// withUnread adds the gap of each file a rule left unread that no gap
// names yet.
func withUnread(gaps []Gap, unread []string) []Gap {
	for _, path := range unread {
		if !slices.ContainsFunc(gaps, func(g Gap) bool { return g.Path == path }) {
			gaps = append(gaps, OverReadBound(path))
		}
	}
	return gaps
}

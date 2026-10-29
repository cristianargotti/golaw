package hook

import (
	"context"
	"errors"
	"io/fs"
	"strings"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
)

// saidKey is the state key of what a session was told about a subject: the
// identity of A15 without the tool use, so the key spans the session. An
// event without a session has no memory, so it says everything again.
func saidKey(repository Repository, event model.Event, subject string) (string, bool) {
	if event.SessionID == "" {
		return "", false
	}
	return Hash([]byte(joinParts([]string{repository.Identity, event.Agent, event.SessionID, event.AgentID, "said", subject}))), true
}

// delivered answers whether the result of the decision carries the
// feedback: one that cannot judge keeps its own answer (apply), so the
// session must not remember lines it never received.
func delivered(decision engine.Decision) bool {
	return decision.Verdict == engine.VerdictClean || decision.Reason != "" && len(decision.Unmeasured) == 0
}

// once drops what the session already received (M6.5.3, M6.6.3): the lines
// of a file at a content it was told about, and the notice of a tool it
// already read. The verdict never depends on it.
func (run execution) once(ctx context.Context, f fed, in model.Input, files []ToolFile, dependencies Dependencies) fed {
	contents := make(map[string][]byte, len(files))
	for _, file := range files {
		contents[file.Path] = file.Content
	}
	told := map[string]bool{}
	kept := make([]model.Finding, 0, len(f.findings))
	for i := range f.findings {
		path := f.findings[i].Path
		seen, asked := told[path]
		if !asked {
			seen = run.seenFile(ctx, path, contents, in, dependencies)
			told[path] = seen
		}
		if !seen {
			kept = append(kept, f.findings[i])
		}
	}
	f.findings = kept
	f.notices, f.person = run.seenTool(ctx, f.notices, f.person, dependencies)
	return f
}

// seenFile answers whether the session got the lines of the path at its
// content: the bytes the tools read, else the snapshot's. A path it cannot
// read keeps its lines.
func (run execution) seenFile(ctx context.Context, path string, contents map[string][]byte, in model.Input, dependencies Dependencies) bool {
	content, ok := contents[path]
	if !ok {
		data, err := in.Snapshot.Read(path)
		if err != nil {
			return false
		}
		content = data
	}
	return run.said(ctx, "lines:"+path+":"+Hash(content), dependencies)
}

// seenTool keeps the notices of the tools the session has not read, each
// with its person sentence; the tool is the first word toolchain writes.
func (run execution) seenTool(ctx context.Context, notices, person []string, dependencies Dependencies) ([]string, []string) {
	var keptNotices, keptPerson []string
	for i, notice := range notices {
		tool, _, _ := strings.Cut(notice, " ")
		if run.said(ctx, "missing:"+tool, dependencies) {
			continue
		}
		keptNotices = append(keptNotices, notice)
		if i < len(person) {
			keptPerson = append(keptPerson, person[i])
		}
	}
	return keptNotices, keptPerson
}

// said answers whether the session was already told the subject, and
// records it when not. A state that cannot be read or written answers
// false, so a line repeats rather than being lost.
func (run execution) said(ctx context.Context, subject string, dependencies Dependencies) bool {
	key, ok := saidKey(run.repository, run.request.Event, subject)
	if !ok {
		return false
	}
	_, err := dependencies.State.Load(ctx, key)
	if err == nil {
		return true
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return false
	}
	// A memory that cannot be written only repeats the line next time.
	if err := dependencies.State.Save(ctx, key, Hash([]byte(subject)), Baseline{}); err != nil {
		return false
	}
	return false
}

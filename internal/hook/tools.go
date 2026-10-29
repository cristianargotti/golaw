package hook

import (
	"github.com/cristianargotti/golaw/internal/model"
	detect "github.com/cristianargotti/golaw/internal/surface"
)

// ToolFile is one file a post event hands to the fast linters: the bytes
// the snapshot reads, so a tool judges what was written, never a later
// state of the disk.
type ToolFile struct {
	Path     string          // slash path relative to the repository root
	Content  []byte          // the bytes the snapshot of the event reads
	Language detect.Language // the surface language detection settled
}

// ToolReport is what the fast linters of a post event answered. A tool it
// could not run is a sentence for the agent in Missing and its sentence in
// the configured language in Person, in the same order, because only the
// binding holds the tool and only the core knows the person's language.
type ToolReport struct {
	Findings []model.Finding // the findings the tools reported, already mapped
	Missing  []string        // one sentence for the agent per tool it could not run
	Person   []string        // the sentence of each entry of Missing for the person
	Killed   []string        // the paths whose tool passed the budget or answered unreadable output
}

// maxToolFiles bounds the files of one event the tools read (M6.6.1: at
// most twenty per cause, the rest counted), so a command that touched a
// whole tree stays within the event budget.
const maxToolFiles = 20

// causeFirstTwenty is the notice of a file past the bound of maxToolFiles.
const causeFirstTwenty = "golaw runs the fast linters on the first twenty files of an event"

// toolFiles are the hand written surface files among the targets of an
// event, in target order, the first twenty kept and the rest named as
// gaps. A file the snapshot or detection cannot read is left out, because
// the checks of the event already name what they could not read.
func toolFiles(in model.Input) ([]ToolFile, []Gap) {
	var files []ToolFile
	var gaps []Gap
	for _, target := range in.Targets {
		content, err := in.Snapshot.Read(target.Path)
		if err != nil {
			continue
		}
		info, err := detect.Detect(target.Path, content)
		if err != nil || info.Language == "" || info.Class != detect.ClassSource {
			continue
		}
		if len(files) == maxToolFiles {
			gaps = append(gaps, Gap{Path: target.Path, Cause: causeFirstTwenty})
			continue
		}
		files = append(files, ToolFile{Path: target.Path, Content: content, Language: info.Language})
	}
	return files, gaps
}

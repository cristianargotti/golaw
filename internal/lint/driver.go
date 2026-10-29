package lint

import (
	"io/fs"
	"time"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/surface"
)

// SuppressedNote ends the message of a finding the source suppressed: the
// suppression is allowed, and the person learns the pipeline may still
// report it.
const SuppressedNote = "; suppressed in the source with a reason, which the pipeline's analyzer may not read"

// ToolTimeout bounds one tool in the lint target, so a hung tool fails CI
// as Unmeasured instead of reaching the job deadline; the hook has its own
// budgets.
const ToolTimeout = 10 * time.Minute

// Input is what a lint target invocation reads.
type Input struct {
	Root string // the repository root
	// Files are slash paths relative to Root: the files of the analyzer
	// scope the driver reads.
	Files []string
	// ConfigDir is the kit configuration of the set in force, materialized
	// in the tool directory, never in the repository (M6.4.1).
	ConfigDir string
	Set       string   // model.SurfaceParity or model.SurfaceStrict
	Globals   []string // surface.scripts.globals
}

// HookInput is what a hook invocation reads; the bytes arrive on stdin.
type HookInput struct {
	// Path is the slash path of the written file, for the file name flag
	// and the report (M6.6.1).
	Path string
	// Shell is the dialect shellcheck reads: sh, bash, dash or ksh, from
	// the shebang or the extension.
	Shell     string
	ConfigDir string
}

// Driver runs one pinned tool.
type Driver struct {
	Tool      string             // a toolchain.Tool name
	Languages []surface.Language // the languages whose files it reads
	Format    Format
	// Args is the lint target invocation, with the repository's own
	// configuration lookup disabled (M6.4.1); nil arguments mean the tool
	// does not run under that set.
	Args func(in Input) []string
	// HookArgs is nil unless the tool may run in the hook (A28).
	HookArgs func(in HookInput) []string
	// Config is the driver's embedded configuration tree, with parity/ and
	// strict/ at its root; nil when the tool needs none.
	Config fs.FS
}

// ToFinding turns an issue into a finding of the report. The Sonar key is
// joined later from the rule map, and a suppressed issue is a note.
func ToFinding(i Issue) model.Finding {
	f := model.Finding{
		Check: "lint." + i.Tool, Rule: i.Tool + "/" + i.Rule, Path: i.Path, Line: i.Line, Column: i.Column,
		Code: Code(i.Rule), Message: i.Message, Severity: model.SeverityFail,
	}
	if i.Suppressed {
		f.Severity, f.Message = model.SeverityNote, i.Message+SuppressedNote
	}
	f.Fingerprint = model.Fingerprint(f)
	return f
}

package surface

import (
	"context"
	"errors"
	"fmt"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
	detect "github.com/cristianargotti/golaw/internal/surface"
)

// rule is the rule reference of the family, the section docs/rules.md
// opens after the inventory's last one.
const rule = "rules/4.16@1"

const (
	goSuffix   = ".go"
	fixtureDir = "testdata"
)

// spec builds the specification of one check of the family. It needs no
// capability, because a language is a fact of the snapshot (A25), never a
// capability a repository declares.
func spec(id string) model.Spec {
	return model.Spec{ID: id, Rule: rule, Scope: model.ScopeFile, Owner: model.OwnerCore, Proof: id}
}

// source is one file under judgment: its path, its bytes and what
// detection knows of it.
type source struct {
	path string
	src  []byte
	info detect.FileInfo
}

// finding builds a finding of the file; Run sets the severity, because it
// depends on the event and not on the predicate.
func (f source) finding(code string, line int, message, sonar string) model.Finding {
	return model.Finding{Path: f.path, Line: line, Code: code, Message: message, Sonar: sonar}
}

// sonarKey prefixes a rule key with the remote analyzer's language key;
// a language the analyzer does not read answers no key at all.
func sonarKey(l detect.Language, key string) string {
	var language string
	switch l {
	case detect.JavaScript, detect.JSX, detect.HTML:
		language = "javascript"
	case detect.TypeScript, detect.TSX:
		language = "typescript"
	case detect.Python:
		language = "python"
	case detect.Dockerfile:
		language = "docker"
	case detect.CloudFormation:
		language = "cloudformation"
	case detect.CSS:
		language = "css"
	case detect.Shell, detect.YAML:
		return ""
	}
	if language == "" {
		return ""
	}
	return language + ":" + key
}

// judge reads one file. It answers no error, because the lexers are total:
// an unterminated literal runs to the end of its unit, so nothing a lexer
// reads leaves a check Unmeasured and a hook never denies a file it could
// not read.
type judge func(f source, in model.Input) []model.Finding

// check is one check of the family: its spec, the languages it judges,
// whether it judges Go files too, whether it is certain (a denial when
// introduced) or a feedback check, whether a feedback check fails a scan
// only under the strict rule set, and whether the introduced rule compares
// codes because the message carries a measure.
type check struct {
	spec      model.Spec
	languages []detect.Language
	goFiles   bool
	certain   bool
	strict    bool
	byCode    bool
	judge     judge
}

// Spec returns the check specification.
func (c check) Spec() model.Spec { return c.spec }

// Run judges every target the check reads. A passed deadline ends the run
// with the findings so far, each established whole, as goast does.
func (c check) Run(ctx context.Context, in model.Input) (model.Result, error) {
	view := withPartial(in)
	severity := c.severity(in)
	var findings []model.Finding
	var unread []string
	measured := 0
	for i := range in.Targets {
		f := &in.Targets[i]
		if err := ctx.Err(); err != nil {
			return model.Result{Findings: findings}, fmt.Errorf("%s: deadline: %w", c.spec.ID, err)
		}
		if !c.reads(f) {
			continue
		}
		src, err := view.Snapshot.Read(f.Path)
		if errors.Is(err, model.ErrTooLarge) {
			unread = append(unread, f.Path)
			continue
		}
		if err != nil {
			return model.Result{Status: model.Unmeasured(err.Error())}, nil
		}
		file := source{path: f.Path, src: src, info: describe(f.Path, src)}
		if !c.judges(file.info) {
			continue
		}
		measured++
		kept := c.introduced(in, file, c.judge(file, in))
		for i := range kept {
			kept[i].Severity = severity
		}
		findings = append(findings, kept...)
	}
	scope := strconv.Itoa(measured) + " files"
	return model.Result{Status: model.Measured(), Scope: scope, Findings: findings, Unread: unread}, nil
}

// reads decides from the snapshot entry alone whether the file may hold
// one of the check's languages: the classes goast judges, never a fixture,
// and a name that settles a language of the check or leaves the content
// to decide among candidates that include one.
func (c check) reads(f *model.File) bool {
	if !judgedClass(f.Class) {
		return false
	}
	if f.Path == fixtureDir || strings.HasPrefix(f.Path, fixtureDir+"/") || strings.Contains(f.Path, "/"+fixtureDir+"/") {
		return false
	}
	if strings.HasSuffix(f.Path, goSuffix) {
		return c.goFiles
	}
	if language, settled := detect.ByName(f.Path); settled {
		return language != "" && slices.Contains(c.languages, language)
	}
	return slices.ContainsFunc(contentCandidates(f.Path), func(l detect.Language) bool {
		return slices.Contains(c.languages, l)
	})
}

// judgedClass reports the snapshot classes goast judges: never a
// generated, fixture, binary or link entry.
func judgedClass(class model.FileClass) bool {
	return class == model.ClassTracked || class == model.ClassUntracked ||
		class == model.ClassTest || class == model.ClassUnreadable
}

// contentCandidates are the languages the bytes of a name ByName leaves
// open may turn out to be: a shebang for a file without an extension, the
// template markers for a data file.
func contentCandidates(name string) []detect.Language {
	switch strings.ToLower(path.Ext(path.Base(name))) {
	case "":
		return []detect.Language{detect.JavaScript, detect.Python, detect.Shell}
	case ".yml", ".yaml":
		return []detect.Language{detect.CloudFormation, detect.YAML}
	}
	return []detect.Language{detect.CloudFormation}
}

// describe answers what detection knows of the file. A YAML or JSON
// document the detector cannot parse is plain YAML to every native check,
// never Unmeasured: the linters of the lint target judge it.
func describe(name string, src []byte) detect.FileInfo {
	info, err := detect.Detect(name, src)
	if errors.Is(err, detect.ErrCloudFormation) {
		return detect.FileInfo{Path: name, Language: detect.YAML, Class: detect.ClassSource}
	}
	return info
}

// judges keeps hand-written files of the check's languages, so minified,
// generated, fixture and declaration files are never judged.
func (c check) judges(info detect.FileInfo) bool {
	if strings.HasSuffix(info.Path, goSuffix) {
		return c.goFiles
	}
	return info.Class == detect.ClassSource && slices.Contains(c.languages, info.Language)
}

// severity is how the findings of this input count. A post event has no
// base to tell an introduced finding from a legacy one, so a certain check
// feeds the finding back there and never asks for a fix of legacy code; a
// feedback check never denies in the hook.
func (c check) severity(in model.Input) model.Severity {
	hook := in.Mode == model.ModeHook
	if c.certain {
		if !hook || writeEvent(in) {
			return model.SeverityDeny
		}
		return model.SeverityNote
	}
	if hook || (c.strict && in.Policy.Surface.Rules != model.SurfaceStrict) {
		return model.SeverityNote
	}
	return model.SeverityFail
}

// writeEvent reports a hook event that is about to write its candidates.
func writeEvent(in model.Input) bool {
	return in.Event != nil && in.Event.Shape == model.ShapeWrite
}

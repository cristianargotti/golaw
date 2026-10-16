package goast

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// rule is the rule reference of the family until docs/rules.md numbers the
// rules in M3: the subsection of the inventory the family ports.
const rule = "rules/4.3@1"

const (
	family     = "goast."
	capability = "go"
	goSuffix   = ".go"
	testSuffix = "_test.go"
	fixtureDir = "testdata"
	cmdDir     = "cmd/"
)

// target is one Go file under judgment: its snapshot entry, its bytes and
// its syntax tree, parsed on first use so a byte-only check never parses.
type target struct {
	file model.File
	src  []byte
	fset *token.FileSet
	tree *ast.File
	err  error
}

// parsed returns the syntax tree and its file set, parsing once; a file
// that does not parse cannot be judged, so the error leaves the check
// Unmeasured.
func (t *target) parsed() (*ast.File, *token.FileSet, error) {
	if t.fset == nil {
		t.fset = token.NewFileSet()
		mode := parser.ParseComments | parser.SkipObjectResolution
		t.tree, t.err = parser.ParseFile(t.fset, t.file.Path, t.src, mode)
		if t.err != nil {
			t.err = parseError(t.file.Path, t.err)
		}
	}
	return t.tree, t.fset, t.err
}

// finding builds a finding of the target at a position; token.NoPos is a
// whole-file finding.
func (t *target) finding(code string, pos token.Pos, message string) model.Finding {
	f := model.Finding{Path: t.file.Path, Code: code, Message: message, Severity: model.SeverityDeny}
	if pos.IsValid() && t.fset != nil {
		position := t.fset.Position(pos)
		f.Line, f.Column = position.Line, position.Column
	}
	return f
}

// dir is the slash separated directory of the target, "." at the root.
func (t *target) dir() string { return path.Dir(t.file.Path) }

// state is what one run shares across its targets: the input, a memo so a
// directory-level finding is reported once, what the files of each
// directory say about its redacting types and the files the run left
// unread.
type state struct {
	in       model.Input
	once     map[string]bool
	redacted map[string]redaction
	unread   []string
}

// read reads a file of the snapshot for the run; a file over the read bound
// is named among the files the run left unread, so the run goes on.
func (s *state) read(name string) ([]byte, error) {
	src, err := s.in.Snapshot.Read(name)
	if errors.Is(err, model.ErrTooLarge) {
		s.leave(name)
	}
	return src, err //nolint:wrapcheck // the snapshot names the path in its error
}

// leave names files over the read bound a verdict of the run depends on
// among the files it left unread, each once.
func (s *state) leave(names ...string) {
	for _, name := range names {
		if !slices.Contains(s.unread, name) {
			s.unread = append(s.unread, name)
		}
	}
}

func newState(in model.Input) *state {
	return &state{in: in, once: map[string]bool{}, redacted: map[string]redaction{}}
}

// first lets a directory-level rule report once per run, however many
// targets its directory holds.
func (s *state) first(key string) bool {
	if s.once[key] {
		return false
	}
	s.once[key] = true
	return true
}

// siblings reads the snapshot and not the targets, so a directory rule
// sees every Go file of the package even when only one file is judged;
// only that directory is asked for, so a hook reads no other.
func (s *state) siblings(dir string) []model.File {
	return model.Select(s.in.Snapshot, func(p string) bool {
		return path.Dir(p) == dir && strings.HasSuffix(p, goSuffix)
	})
}

// judge inspects one target; an error means the target could not be
// judged and the check answers Unmeasured.
type judge func(t *target, s *state) ([]model.Finding, error)

// gate answers whether the snapshot meets a check's condition and, when it
// does not, the NotApplicable status to record.
type gate func(in model.Input) (model.Status, bool)

// check is one file-scoped check of the family: its spec, whether test
// files and generated files are judged, an optional gate and the judge.
// Run is written once so every check reads, parses and fails the same way.
type check struct {
	spec      model.Spec
	tests     bool
	generated bool
	gate      gate
	judge     judge
}

// Spec returns the check specification.
func (c check) Spec() model.Spec { return c.spec }

// Run judges every applicable target; a read or parse failure leaves the
// check Unmeasured, never clean, and a passed deadline is an error the
// engine records the same way.
func (c check) Run(ctx context.Context, in model.Input) (model.Result, error) {
	if c.gate != nil {
		if status, ok := c.gate(in); !ok {
			return model.Result{Status: status}, nil
		}
	}
	st := newState(in)
	var findings []model.Finding
	measured := 0
	for _, f := range in.Targets {
		if err := ctx.Err(); err != nil {
			return model.Result{}, fmt.Errorf("%s: deadline: %w", c.spec.ID, err)
		}
		if !c.applies(f) {
			continue
		}
		src, err := st.read(f.Path)
		if errors.Is(err, model.ErrTooLarge) {
			continue
		}
		if err != nil {
			return model.Result{Status: model.Unmeasured(err.Error())}, nil
		}
		found, err := c.judge(&target{file: f, src: src}, st)
		if err != nil {
			return model.Result{Status: model.Unmeasured(err.Error())}, nil
		}
		measured++
		findings = append(findings, found...)
	}
	scope := strconv.Itoa(measured) + " files"
	return model.Result{Status: model.Measured(), Scope: scope, Findings: findings, Unread: st.unread}, nil
}

// applies reports whether a target is a Go file the check judges: tracked
// and untracked sources always, test files when the check keeps them,
// generated files when it counts them, and never a fixture, a binary or a
// link. The fixture and test rules read the path, because a generated or an
// unreadable file has no class that says so; an unreadable file that passes
// them applies, and its failed read leaves the check Unmeasured instead of
// clean.
func (c check) applies(f model.File) bool {
	if !strings.HasSuffix(f.Path, goSuffix) || fixturePath(f.Path) {
		return false
	}
	if strings.HasSuffix(f.Path, testSuffix) && !c.tests {
		return false
	}
	if f.Class == model.ClassGenerated {
		return c.generated
	}
	return f.Class == model.ClassTracked || f.Class == model.ClassUntracked ||
		f.Class == model.ClassTest || f.Class == model.ClassUnreadable
}

// fixturePath applies the testdata rule source classifies fixtures by to
// the path alone, so it holds for a file whose bytes were never read.
func fixturePath(p string) bool {
	return strings.HasPrefix(p, fixtureDir+"/") || strings.Contains(p, "/"+fixtureDir+"/")
}

// spec builds the specification of one check of the family: the id is the
// family plus the name, the rule is the family's, the scope is file, the
// owner is core, the proof key is the id, and go is always needed.
func spec(name string, needs ...string) model.Spec {
	id := family + name
	return model.Spec{
		ID:    id,
		Rule:  rule,
		Scope: model.ScopeFile,
		Owner: model.OwnerCore,
		Needs: append([]string{capability}, needs...),
		Proof: id,
	}
}

// requireFile asks the snapshot for the class of a file at the root, so
// the gate answers without a read.
func requireFile(name string) gate {
	return func(in model.Input) (model.Status, bool) {
		if in.Snapshot.Class(name) == model.ClassUnknown {
			return model.NotApplicable(name + " is absent"), false
		}
		return model.Status{}, true
	}
}

// requireDir looks for a file below the directory, because a snapshot
// lists files and never directories; the presence of a path is all it
// asks, so no file is read.
func requireDir(dir string) gate {
	return func(in model.Input) (model.Status, bool) {
		if model.Any(in.Snapshot, func(p string) bool { return strings.HasPrefix(p, dir+"/") }) {
			return model.Status{}, true
		}
		return model.NotApplicable(dir + " is absent"), false
	}
}

// parseError names the path, because the error becomes the Unmeasured
// reason a reader acts on.
func parseError(name string, err error) error {
	return fmt.Errorf("parse %s: %w", name, err)
}

// under matches each prefix at the start of the path only, so cmd/ never
// matches internal/cmd/.
func under(p string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
}

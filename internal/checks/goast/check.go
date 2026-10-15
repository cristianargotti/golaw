package goast

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
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
// directory-level finding is reported once, and the redacting types found
// per directory.
type state struct {
	in       model.Input
	once     map[string]bool
	redacted map[string]map[string]bool
}

func newState(in model.Input) *state {
	return &state{in: in, once: map[string]bool{}, redacted: map[string]map[string]bool{}}
}

// first reports whether key is seen for the first time in the run.
func (s *state) first(key string) bool {
	if s.once[key] {
		return false
	}
	s.once[key] = true
	return true
}

// siblings lists the Go files of one directory of the snapshot.
func (s *state) siblings(dir string) []model.File {
	var out []model.File
	for _, f := range s.in.Snapshot.Files() {
		if path.Dir(f.Path) == dir && strings.HasSuffix(f.Path, goSuffix) {
			out = append(out, f)
		}
	}
	return out
}

// judge inspects one target; an error means the target could not be
// judged and the check answers Unmeasured.
type judge func(t *target, s *state) ([]model.Finding, error)

// gate answers whether the snapshot meets a check's condition and, when it
// does not, the NotApplicable status to record.
type gate func(in model.Input) (model.Status, bool)

// check is one file-scoped check of the family: its spec, whether test
// files are judged, an optional gate and the judge. Run is written once so
// every check reads, parses and fails the same way.
type check struct {
	spec  model.Spec
	tests bool
	gate  gate
	judge judge
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
		src, err := in.Snapshot.Read(f.Path)
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
	return model.Result{Status: model.Measured(), Scope: scope, Findings: findings}, nil
}

// applies reports whether a target is a Go file the check judges: tracked
// and untracked sources always, test files when the check keeps them, and
// never a fixture, a generated file or a non-text class.
func (c check) applies(f model.File) bool {
	if !strings.HasSuffix(f.Path, goSuffix) {
		return false
	}
	if f.Class == model.ClassTest {
		return c.tests
	}
	return f.Class == model.ClassTracked || f.Class == model.ClassUntracked
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

// requireFile gates a check on a file at the snapshot root.
func requireFile(name string) gate {
	return func(in model.Input) (model.Status, bool) {
		if in.Snapshot.Class(name) == model.ClassUnknown {
			return model.NotApplicable(name + " is absent"), false
		}
		return model.Status{}, true
	}
}

// requireDir gates a check on a directory of the snapshot.
func requireDir(dir string) gate {
	return func(in model.Input) (model.Status, bool) {
		for _, f := range in.Snapshot.Files() {
			if strings.HasPrefix(f.Path, dir+"/") {
				return model.Status{}, true
			}
		}
		return model.NotApplicable(dir + " is absent"), false
	}
}

// parseError wraps a parse failure with the path it happened in.
func parseError(name string, err error) error {
	return fmt.Errorf("parse %s: %w", name, err)
}

// under reports whether a path sits below one of the root-level prefixes.
func under(p string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
}

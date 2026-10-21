package policyfiles

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// foreignMessage names both ways out, because without an actor identity the
// check cannot tell a member of the owning team from anyone else.
const foreignMessage = "%s belongs to %s under the base %s; leave it out of this commit and ask the owning team " +
	"to change this path, or, as a member of that team, set pre_commit: false under ladder in golaw.yml for this commit"

type codeownersForeign struct{}

// CodeownersForeign uses base assignments so a staged edit cannot authorize itself.
func CodeownersForeign() model.Check { return codeownersForeign{} }

func (codeownersForeign) Spec() model.Spec { return spec("codeowners-foreign", ruleDelivery) }

func (codeownersForeign) Run(_ context.Context, in model.Input) (model.Result, error) {
	if status, applies := profiled(in); !applies {
		return model.Result{Status: status}, nil
	}
	if in.Mode != model.ModeStaged || len(in.Changes) == 0 {
		return model.Result{Status: model.NotApplicable("no staged change to judge")}, nil
	}
	path, rules, err := baseOwnership(in)
	if err != nil {
		return answer(path, err), nil
	}
	if path == "" {
		return model.Result{Status: model.NotApplicable("no CODEOWNERS file at the staged base")}, nil
	}
	return model.Result{
		Status: model.Measured(), Scope: "staged paths against base " + path,
		Findings: foreignChanges(path, rules, in.Changes),
	}, nil
}

// foreignChanges names the base assignment and remedy so a finding is actionable
// even when the snapshot cannot identify a member of the owning team.
func foreignChanges(base string, rules owners, changes []model.Change) []model.Finding {
	var findings []model.Finding
	for _, path := range ownershipPaths(changes) {
		foreign, line := rules.foreign(path)
		if !foreign {
			continue
		}
		assigned := strings.Join(rules.assignment(path).owners, " ")
		message := fmt.Sprintf(foreignMessage, path, assigned, base)
		findings = append(findings, finding(base, line, "foreign-owned-change", message))
	}
	return findings
}

// ownershipPaths includes both rename endpoints so moving a foreign path cannot
// evade its assignment, and deduplicates paths to avoid repeated findings.
func ownershipPaths(changes []model.Change) []string {
	var paths []string
	for _, change := range changes {
		switch change.Op {
		case model.ChangeAdd, model.ChangeModify:
			paths = append(paths, change.NewPath)
		case model.ChangeDelete:
			paths = append(paths, change.OldPath)
		case model.ChangeRename:
			paths = append(paths, change.OldPath, change.NewPath)
		}
	}
	slices.Sort(paths)
	return slices.Compact(paths)
}

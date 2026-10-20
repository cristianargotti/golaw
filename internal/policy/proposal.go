package policy

import (
	"fmt"

	"github.com/cristianargotti/golaw/internal/model"
)

// StagedPolicy is what an index holds at golaw.yml, as the index snapshot
// read it, so the proposal is judged on the same frozen bytes as every other
// staged file.
type StagedPolicy struct {
	Data     []byte // the stage 0 blob, nil when Found is false
	Found    bool   // stage 0 holds golaw.yml
	Unmerged bool   // stages 1 to 3 hold it: a merge left it unresolved
}

// EvaluateProposal judges the next policy an index holds apart from the
// policy in force (A2): the proposal when it differs from the policy in
// force; ErrPolicy (model) for an unresolved merge of golaw.yml, a file that
// breaks the schema or a weakening (CheckTightening); ErrEnrollment (model)
// for a staged deletion of the golaw.yml HEAD holds on an enrolled
// repository (the rule cmd/golaw/bind_hook.go applies as stagedDeletion,
// which keeps working unchanged). The reason names the key that weakens and
// the fix: unstage golaw.yml or restore the value.
func EvaluateProposal(inForce *Loaded, staged StagedPolicy, schemas Schemas) (Proposal, error) {
	switch {
	case staged.Unmerged:
		return Proposal{Staged: true}, unresolved()
	case !staged.Found:
		return Proposal{}, inForce.deletion()
	}
	proposal, err := inForce.judge(staged.Data, schemas)
	if err != nil {
		return proposal, fmt.Errorf("%w: restore what the policy in force holds, or unstage %s with git restore --staged %s", err, FileName, FileName)
	}
	return proposal, nil
}

// judge reads a staged golaw.yml as the next policy (A2). It is validated
// before its digest is compared, so a file that hashes like the policy in
// force is still refused when it breaks the schema; the bytes of the policy
// in force are no proposal, and anything else must tighten it, unless the
// core defaults are in force and validation alone applies.
func (l *Loaded) judge(data []byte, schemas Schemas) (Proposal, error) {
	next, err := Decode(data)
	if err == nil {
		err = next.Validate(schemas)
	}
	if err != nil {
		return Proposal{Staged: true}, err
	}
	if Digest(next) == l.Policy.Digest {
		return Proposal{Staged: true}, nil
	}
	if l.Origin != OriginCore {
		if err := CheckTightening(l.File, next, schemas); err != nil {
			return Proposal{Staged: true}, err
		}
	}
	return Proposal{File: next, Found: true, Staged: true}, nil
}

// deletion refuses an index without golaw.yml while HEAD holds the policy
// an enrolled repository is governed by (A3): the next commit would delete
// it. An unenrolled repository may delete its file, and before the first
// commit the bootstrap record governs, so neither is refused.
func (l *Loaded) deletion() error {
	if l.Origin != OriginHead || l.Record == nil {
		return nil
	}
	return fmt.Errorf("%w: the deletion of %s is staged in %s (%s): restore it with git restore --staged --worktree %s",
		model.ErrEnrollment, FileName, l.Root, model.EnrollmentMissing, FileName)
}

// unresolved is the refusal of a golaw.yml a merge left at stages 1 to 3,
// which holds no proposed policy until the merge is finished.
func unresolved() error {
	return fmt.Errorf("%w: %s is unresolved in the index: finish the merge before the policy is judged", model.ErrPolicy, FileName)
}

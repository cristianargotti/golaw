// Package stage judges what a commit introduces: the index snapshot of the
// next commit, the proposed policy judged apart (A2), the rules over the
// staged files and, compared as a multiset of fingerprints with the same
// files at the base and, during a merge, at the other parent, only the
// findings the index introduces.
package stage

import (
	"context"
	"fmt"
	"slices"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/source"
)

// Input is what a staged scan needs; the caller loads the policy in force
// (the pin of A10 and the enrollment checked) and discovers the repository.
type Input struct {
	Loaded   *policy.Loaded
	Repo     source.Repo
	Index    source.IndexOptions
	Registry *engine.Registry
	Schemas  policy.Schemas
}

// Result is one staged scan.
type Result struct {
	Input    model.Input     // ModeStaged over the index, as dispatched
	Index    *source.Index   // the snapshot, for the commit rung that judges the same index
	Run      *engine.Run     // the run with each outcome keeping only the findings the index introduces
	Decision engine.Decision // engine.Decide over that run
	Proposal policy.Proposal // the golaw.yml the index holds, judged apart
	Brought  []string        // the paths a merge in progress brought, left to the pipeline, sorted
}

// Scan builds the index snapshot, judges the proposed policy
// (policy.EvaluateProposal over the golaw.yml the snapshot holds), leaves
// out what a merge in progress brought (Brought), dispatches the registry in
// ModeStaged over the staged files under the policy in force, dispatches it
// again over the same paths at the base (source.NewRevision of Base, the old
// path of a rename) and apart over the files beside a target whose
// directory those paths miss, the same at the commit MERGE_HEAD names while
// a merge is in progress, and keeps the findings report.Introduced names;
// with a base the input names, it compares with that commit alone. A
// refused proposal is its error (ErrPolicy or ErrEnrollment of model), so
// the command exits 2.
func Scan(ctx context.Context, in Input) (Result, error) {
	index, err := source.NewIndex(ctx, in.Repo, in.Index)
	if err != nil {
		return Result{}, fmt.Errorf("freeze the index of %s: %w", in.Repo.Root, err)
	}
	staged, err := stagedPolicy(index)
	if err != nil {
		return Result{}, err
	}
	proposal, err := policy.EvaluateProposal(in.Loaded, staged, in.Schemas)
	if err != nil {
		return Result{}, fmt.Errorf("staged %s: %w", policy.FileName, err)
	}
	brought, theirs, err := mergeOf(ctx, in, index)
	if err != nil {
		return Result{}, err
	}
	candidate := leftOut(model.Input{
		Mode: model.ModeStaged, Policy: in.Loaded.Policy, Snapshot: index, Targets: index.Staged(), Changes: index.Changes(),
	}, brought)
	run, err := engine.Dispatch(ctx, in.Registry, candidate)
	if err != nil {
		return Result{}, fmt.Errorf("judge the index of %s: %w", in.Repo.Root, err)
	}
	if err := introducedOnly(ctx, in, candidate, run, theirs); err != nil {
		return Result{}, err
	}
	return Result{Input: candidate, Index: index, Run: run, Decision: engine.Decide(run), Proposal: proposal, Brought: brought}, nil
}

// stagedPolicy is what the index snapshot holds at golaw.yml, read from the
// same frozen blobs as every staged file; a staged golaw.yml the snapshot
// cannot read, such as a link, is ErrPolicy.
func stagedPolicy(index *source.Index) (policy.StagedPolicy, error) {
	if slices.Contains(index.Unmerged(), policy.FileName) {
		return policy.StagedPolicy{Unmerged: true}, nil
	}
	if index.Class(policy.FileName) == model.ClassUnknown {
		return policy.StagedPolicy{}, nil
	}
	data, err := index.Read(policy.FileName)
	if err != nil {
		return policy.StagedPolicy{}, fmt.Errorf("%w: read the staged %s: %w", model.ErrPolicy, policy.FileName, err)
	}
	return policy.StagedPolicy{Data: data, Found: true}, nil
}

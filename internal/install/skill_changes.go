package install

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/cristianargotti/golaw/internal/assets"
)

// addSharedSkill plans the shared skill from its one source. A skill on
// disk is replaced only when it is absent, already the source, or bytes
// an earlier kit wrote that a receipt of the scope records, which is the
// upgrade and the rollback to an earlier tag; anything else is the
// person's own skill and is refused, never overwritten. Every other
// receipt that lists the skill is rewritten with the digest of the
// source, so each owner keeps recording what is on disk and its doctor
// and uninstall read the file as golaw's.
func addSharedSkill(plan *Plan, options Options) error {
	source, err := assets.Read(skillSource)
	if err != nil {
		return fmt.Errorf("load the shared skill: %w", err)
	}
	path := SkillPath(options)
	current, exists, err := readOptional(path)
	if err != nil {
		return err
	}
	receipts, err := skillReceipts(options.StateDir, options.Scope, plan.Receipt.ProjectID, path)
	if err != nil {
		return err
	}
	if exists && !bytes.Equal(current, source) && !recordsDigest(receipts, path, Digest(current)) {
		return changedArtifact(path, ErrChanged)
	}
	if err := addArtifact(plan, path, source); err != nil {
		return err
	}
	return refreshSkillOwners(plan, receipts, path, Digest(source))
}

// removeSharedSkill plans the removal of the shared skill a receipt lists,
// only when no receipt of another agent at the scope still lists it. The
// bytes on disk must be golaw's: the digest this receipt recorded, the
// current source, which another install may have upgraded the file to, or
// the digest another owner's receipt records.
func removeSharedSkill(plan *Plan, stateDir, agent string, artifact Artifact) error {
	current, exists, err := readOptional(artifact.Path)
	if err != nil || !exists {
		return err
	}
	scope, identity := plan.Receipt.Scope, plan.Receipt.ProjectID
	digest := Digest(current)
	if digest != artifact.SHA256 {
		known, knownErr := skillDigestKnown(stateDir, scope, identity, artifact.Path, digest)
		if knownErr != nil {
			return knownErr
		}
		if !known {
			return changedArtifact(artifact.Path, ErrChanged)
		}
	}
	owners, err := SkillOwners(stateDir, scope, identity, agent, artifact.Path)
	if err != nil || len(owners) > 0 {
		return err
	}
	return addChange(plan, artifact.Path, nil, true)
}

func skillDigestKnown(stateDir, scope, identity, path, digest string) (bool, error) {
	source, err := assets.Read(skillSource)
	if err != nil {
		return false, fmt.Errorf("load the shared skill: %w", err)
	}
	if digest == Digest(source) {
		return true, nil
	}
	receipts, err := skillReceipts(stateDir, scope, identity, path)
	if err != nil {
		return false, err
	}
	return recordsDigest(receipts, path, digest), nil
}

func recordsDigest(receipts map[string]Receipt, path, digest string) bool {
	recorded := Artifact{Path: path, SHA256: digest}
	for receiptPath := range receipts {
		if slices.Contains(receipts[receiptPath].Artifacts, recorded) {
			return true
		}
	}
	return false
}

// refreshSkillOwners rewrites, in path order so the diff reads the same on
// every run, each receipt other than the one being planned whose record
// of the skill differs from digest; the planner writes its own receipt.
func refreshSkillOwners(plan *Plan, receipts map[string]Receipt, path, digest string) error {
	paths := make([]string, 0, len(receipts))
	for receiptPath := range receipts {
		paths = append(paths, receiptPath)
	}
	slices.Sort(paths)
	for _, receiptPath := range paths {
		if receiptPath == plan.ReceiptPath {
			continue
		}
		receipt, stale := refreshedReceipt(receipts[receiptPath], path, digest)
		if !stale {
			continue
		}
		encoded, err := json.MarshalIndent(receipt, "", "  ")
		if err != nil {
			return fmt.Errorf("encode receipt: %w", err)
		}
		if err := addChange(plan, receiptPath, append(encoded, '\n'), false); err != nil {
			return err
		}
	}
	return nil
}

// refreshedReceipt answers receipt with digest recorded for path, and
// whether any record of path held another digest.
func refreshedReceipt(receipt Receipt, path, digest string) (Receipt, bool) {
	receipt.Artifacts = slices.Clone(receipt.Artifacts)
	stale := false
	for index := range receipt.Artifacts {
		if receipt.Artifacts[index].Path == path && receipt.Artifacts[index].SHA256 != digest {
			receipt.Artifacts[index].SHA256, stale = digest, true
		}
	}
	return receipt, stale
}

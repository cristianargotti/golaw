package main

import (
	"context"
	"fmt"
	"time"

	"github.com/cristianargotti/golaw/internal/checks/paths"
	"github.com/cristianargotti/golaw/internal/hook/agents"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

// installOptions is the installOptions seam: the host directories outside
// the checkout, the version of the binary, the digests of the policy in
// force and the read deny rules. A repository without a policy in force is
// refused, because the hooks it would arm enforce enrolled repositories
// only (A3).
func (b binding) installOptions(ctx context.Context, agent agents.Agent, root, scope string, now time.Time) (install.Options, error) {
	l, err := b.loaded(ctx, root, now)
	if err != nil {
		return install.Options{}, err
	}
	if l.Origin == policy.OriginCore {
		return install.Options{}, fmt.Errorf("%w: %s has no %s in force: run init --apply first", model.ErrUsage, root, policy.FileName)
	}
	home, err := b.home()
	if err != nil {
		return install.Options{}, fmt.Errorf("resolve the home directory: %w", err)
	}
	state, err := b.installState()
	if err != nil {
		return install.Options{}, err
	}
	rules, err := install.ReadRules(paths.SecretGlobs())
	if err != nil {
		return install.Options{}, fmt.Errorf("build the read deny rules: %w", err)
	}
	return install.Options{
		Root: root, ConfigDir: agent.ConfigDir(b.env, home), StateDir: state, Scope: scope,
		Version: b.info.Version, PolicyDigest: l.Policy.Digest, ProfileDigest: profileDigest(l.File),
		ReadRules: rules,
	}, nil
}

// receiptPath is the receiptPath seam: where the receipt of an agent lives
// for a repository and a scope.
func (b binding) receiptPath(agent agents.Agent, root, scope string) (string, error) {
	state, err := b.installState()
	if err != nil {
		return "", err
	}
	_, identity, err := install.ProjectIdentity(root)
	if err != nil {
		return "", fmt.Errorf("identify %s: %w", root, err)
	}
	return agents.ReceiptPath(state, agent.ID, scope, identity), nil
}

// enrollmentRecord is the enrollment record of A3 behind the ports of the
// installer and doctor, keyed by the real root of the repository.
type enrollmentRecord struct {
	stateDir string
	root     string
	version  string
	now      time.Time
}

// enrollments is the enrollments seam.
func (b binding) enrollments(root string, now time.Time) (enrollmentPort, error) {
	dirs, err := b.dirs()
	if err != nil {
		return nil, err
	}
	return enrollmentRecord{stateDir: dirs.State, root: root, version: b.info.Version, now: now}, nil
}

// WriteEnrollment writes the record when none holds the digests already,
// keeping the bootstrap policy an earlier init wrote.
func (r enrollmentRecord) WriteEnrollment(_ context.Context, identity, policyDigest, profileDigest string) (model.Enrollment, error) {
	if err := r.verify(identity); err != nil {
		return model.Enrollment{}, err
	}
	enrolled := model.Enrollment{State: model.EnrollmentEnrolled, PolicyDigest: policyDigest, ProfileDigest: profileDigest}
	existing, found, err := policy.ReadRecord(r.stateDir, r.root)
	if err != nil {
		return model.Enrollment{}, fmt.Errorf("read the enrollment record: %w", err)
	}
	if found && existing.PolicyDigest == policyDigest && existing.ProfileDigest == profileDigest {
		return enrolled, nil
	}
	record := policy.Record{
		Root: r.root, PolicyDigest: policyDigest, ProfileDigest: profileDigest, KitVersion: r.version,
		Bootstrap: existing.Bootstrap, WrittenBy: policy.WrittenByInstall, WrittenAt: r.now,
	}
	if err := policy.WriteRecord(r.stateDir, record); err != nil {
		return model.Enrollment{}, fmt.Errorf("write the enrollment record: %w", err)
	}
	return enrolled, nil
}

// ReadEnrollment answers the four states of A3 against the digests in
// force: no record, a matching record, a record whose digests differ, and
// a record for a repository whose policy file is gone. Gone means no
// policy in force or no file in the checkout, the same file the hook and
// scan refuse to run without, so doctor never reports enrolled where they
// deny.
func (r enrollmentRecord) ReadEnrollment(_ context.Context, identity, policyDigest, profileDigest string) (model.Enrollment, error) {
	if err := r.verify(identity); err != nil {
		return model.Enrollment{}, err
	}
	record, found, err := policy.ReadRecord(r.stateDir, r.root)
	if err != nil {
		return model.Enrollment{}, fmt.Errorf("read the enrollment record: %w", err)
	}
	if !found {
		return model.Enrollment{State: model.EnrollmentNone}, nil
	}
	present, err := inCheckout(r.root)
	if err != nil {
		return model.Enrollment{}, err
	}
	e := model.Enrollment{State: model.EnrollmentEnrolled, PolicyDigest: record.PolicyDigest, ProfileDigest: record.ProfileDigest}
	switch {
	case policyDigest == "" || !present:
		e.State = model.EnrollmentMissing
	case record.PolicyDigest != policyDigest || record.ProfileDigest != profileDigest:
		e.State = model.EnrollmentMismatch
	}
	return e, nil
}

// verify refuses an identity that is not the repository the record is for.
func (r enrollmentRecord) verify(identity string) error {
	_, want, err := install.ProjectIdentity(r.root)
	if err != nil {
		return fmt.Errorf("identify %s: %w", r.root, err)
	}
	if identity != want {
		return fmt.Errorf("%w: identity %s is not the repository %s", model.ErrEnrollment, identity, r.root)
	}
	return nil
}

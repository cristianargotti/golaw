package main

import (
	"context"
	"fmt"
	"time"

	"github.com/cristianargotti/golaw/internal/checks/paths"
	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/hook/agents"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/policy"
)

// doctorReport is the doctorReport seam: the evidence doctor collects for
// an agent on a repository, assessed.
func (b binding) doctorReport(ctx context.Context, agent agents.Agent, root, scope string, now time.Time) (doctor.Report, error) {
	input, err := b.doctorInput(ctx, agent, root, scope, now)
	if err != nil {
		return doctor.Report{}, err
	}
	port, err := b.enrollments(root, now)
	if err != nil {
		return doctor.Report{}, err
	}
	return doctor.Assess(doctor.Collect(ctx, input, port, doctor.NativeRunner{})), nil
}

// doctorInput gathers what doctor needs: the receipt, the running binary,
// the digests and the pin of the policy in force, the read deny rules and
// the environment a hook process observed.
func (b binding) doctorInput(ctx context.Context, agent agents.Agent, root, scope string, now time.Time) (doctor.Input, error) {
	receipt, err := b.receiptPath(agent, root, scope)
	if err != nil {
		return doctor.Input{}, err
	}
	running, err := b.exe()
	if err != nil {
		return doctor.Input{}, fmt.Errorf("resolve the running binary: %w", err)
	}
	rules, err := install.ReadRules(paths.SecretGlobs())
	if err != nil {
		return doctor.Input{}, fmt.Errorf("build the read deny rules: %w", err)
	}
	input := doctor.Input{Root: root, ReceiptPath: receipt, RunningBinary: running, RequiredRead: rules}
	input.PolicyDigest, input.ProfileDigest, input.PinnedVersion = b.policyFacts(ctx, root, now)
	b.observed(agent, root, &input)
	return input, nil
}

// policyFacts answers the digests and the pin of the policy in force, from
// the loaded policy when it loads and from the file at HEAD otherwise, so a
// mismatch is reported as such instead of hiding behind a load error.
func (b binding) policyFacts(ctx context.Context, root string, now time.Time) (string, string, string) {
	if l, err := b.loaded(ctx, root, now); err == nil && l.Origin != policy.OriginCore {
		return l.Policy.Digest, profileDigest(l.File), l.Policy.KitVersion
	}
	data, found, err := policy.ReadHead(ctx, root)
	if err != nil || !found {
		return "", "", ""
	}
	f, err := policy.Decode(data)
	if err != nil {
		return "", "", ""
	}
	return policy.Digest(f), profileDigest(f), f.Kit.Version
}

// observed fills the environment a hook process of the agent recorded for
// the repository, when one exists for the current hook definition.
func (b binding) observed(agent agents.Agent, root string, input *doctor.Input) {
	state, err := b.installState()
	if err != nil {
		return
	}
	_, identity, err := install.ProjectIdentity(root)
	if err != nil {
		return
	}
	digest, err := hookDigest(agent)
	if err != nil {
		return
	}
	env, err := doctor.LoadObservation(agents.ObservationPath(state, agent.ID, identity), identity, digest)
	if err == nil {
		input.Environment = env
	}
}

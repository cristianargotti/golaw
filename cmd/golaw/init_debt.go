package main

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"path"
	"slices"
	"time"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/model"
)

// The seeded set expires 180 days after the adoption (A24, for the owner),
// and its record sits beside the inherited baseline.
const (
	debtHorizonDays = 180
	debtRecordName  = "0002-inherited-debt.md"
	debtDay         = "2006-01-02"
)

// seedRun is what one seed judges and writes: the worktree with the files
// init --apply writes folded in, the policy those files hold compiled, the
// catalog, what the scan found, the measurements the person gave, the
// record path and the days.
type seedRun struct {
	writes                 map[string][]byte
	policy                 *model.EffectivePolicy
	snapshot               model.Snapshot
	registry               *engine.Registry
	findings               []model.Finding
	exemptions, advisories []string
	record, until, day     string
}

// checkSeedFlags refuses the flag combinations the seed cannot honor.
func checkSeedFlags(opts initOptions) error {
	switch {
	case opts.seedDebt && opts.update:
		return fmt.Errorf("%w: --seed-debt seeds a first adoption, while init --update keeps the policy and moves the kit pin", model.ErrUsage)
	case !opts.seedDebt && (opts.coverageProfile != "" || opts.vulnerabilities != ""):
		return fmt.Errorf("%w: --coverage-profile and --vulnerabilities feed --seed-debt; give it with them", model.ErrUsage)
	}
	return nil
}

// seedPlan seeds the reviewed debt set into the plan of a first adoption
// (A24): refused once a policy or an enrollment exists, then the set
// applied to the policy init writes, its record among the writes, the
// policy compiled again and one line that counts what was seeded.
func (a *app) seedPlan(ctx context.Context, root string, plan initPlan, opts initOptions) (initPlan, error) {
	if !opts.seedDebt {
		return plan, nil
	}
	if err := a.bind().seedOnce(ctx, root, plan.current); err != nil {
		return initPlan{}, err
	}
	data, record, err := a.seedInitDebt(ctx, root, plan, opts, a.now())
	if err != nil {
		return initPlan{}, err
	}
	plan.debt = debtLine(plan.policy.Language, data, record)
	if record.Path == "" {
		return plan, nil
	}
	plan.data = data
	plan.templates.Changes = append(slices.Clone(plan.templates.Changes), record)
	slices.SortFunc(plan.templates.Changes, func(x, y install.Change) int { return cmp.Compare(x.Path, y.Path) })
	plan.policy, err = a.compile(plan.data, a.now())
	return plan, err
}

// seedInitDebt judges the tree init --apply leaves under the policy it
// writes, reads the measurements the person gave, and answers the policy
// with the set applied and the record as a write; a tree without debt
// answers the policy unchanged and no record.
func (a *app) seedInitDebt(ctx context.Context, root string, plan initPlan, opts initOptions, now time.Time) ([]byte, install.Change, error) {
	s, err := a.seedScan(ctx, root, plan, now)
	if err == nil {
		err = a.bind().seedRecordAbsent(root, s.record)
	}
	if err != nil {
		return nil, install.Change{}, err
	}
	if s.exemptions, err = coverageExemptions(opts.coverageProfile, root, s.policy); err != nil {
		return nil, install.Change{}, err
	}
	if s.advisories, err = reachableAdvisories(opts.vulnerabilities); err != nil {
		return nil, install.Change{}, err
	}
	debt := bootstrap.SeedDebt(s.findings, s.exemptions, s.advisories, s.record, s.until)
	if debt.Empty() {
		return plan.data, install.Change{}, nil
	}
	seeded, record, err := settleDebt(ctx, s, plan.data, debt)
	if err != nil {
		return nil, install.Change{}, err
	}
	return seeded, install.Change{Path: s.record, After: record}, nil
}

// seedScan compiles the policy init writes and dispatches the catalog in
// scan mode over the worktree with every file init --apply writes folded
// in, so a finding a written template fixes is never seeded.
func (a *app) seedScan(ctx context.Context, root string, plan initPlan, now time.Time) (seedRun, error) {
	compiled, err := a.bind().seedPolicy(plan.data, now)
	if err != nil {
		return seedRun{}, err
	}
	snapshot, err := a.snapshot(ctx, root)
	if err != nil {
		return seedRun{}, err
	}
	registry, err := a.registry()
	if err != nil {
		return seedRun{}, err
	}
	s := seedRun{
		writes: map[string][]byte{bootstrap.PolicyFile: plan.data}, policy: compiled, snapshot: snapshot, registry: registry,
		record: path.Join(compiled.Layout.Decisions, debtRecordName),
		until:  now.UTC().AddDate(0, 0, debtHorizonDays).Format(debtDay), day: now.UTC().Format(debtDay),
	}
	for _, change := range plan.templates.Changes {
		s.writes[change.Path] = change.After
	}
	s.findings, err = seedFindings(ctx, s, s.writes, nil)
	return s, err
}

// settleDebt applies the set and, when golaw.yml or the record would raise
// a finding themselves (a seeded path copied into them), seeds those
// findings too, so the adoption writes no finding its seed did not record;
// the entries the second pass adds name only those two files, which raise
// nothing more, and a third pass that still differs is an error.
func settleDebt(ctx context.Context, s seedRun, rendered []byte, debt bootstrap.Debt) ([]byte, []byte, error) {
	for range 3 {
		data, err := bootstrap.ApplyDebt(rendered, debt)
		if err != nil {
			return nil, nil, initOperation("seed the debt set", err)
		}
		record := bootstrap.DebtRecord(debt, s.day, s.policy.Language)
		files := maps.Clone(s.writes)
		files[bootstrap.PolicyFile], files[s.record] = data, record
		written, err := seedFindings(ctx, s, files, []string{bootstrap.PolicyFile, s.record})
		if err != nil {
			return nil, nil, err
		}
		next := bootstrap.SeedDebt(append(slices.Clone(s.findings), written...), s.exemptions, s.advisories, s.record, s.until)
		if slices.Equal(next.Exceptions, debt.Exceptions) {
			return data, record, nil
		}
		debt = next
	}
	return nil, nil, fmt.Errorf("%w: the seeded %s keeps raising findings at itself", model.ErrPolicy, bootstrap.PolicyFile)
}

// debtLine counts what the seed wrote, in the language of the policy.
func debtLine(language string, data []byte, record install.Change) string {
	if record.Path == "" {
		if language == model.DefaultLanguage {
			return "Nenhuma dívida a semear: golaw.yml começa sem exceções, isenções ou avisos."
		}
		return "No debt to seed: golaw.yml starts without exceptions, exemptions or waivers."
	}
	counts := debtCounts(data)
	if language == model.DefaultLanguage {
		return fmt.Sprintf("Dívida semeada em golaw.yml (exceções: %d, isenções de cobertura: %d, avisos de vulnerabilidade: %d), registrada em %s.",
			counts[0], counts[1], counts[2], record.Path)
	}
	return fmt.Sprintf("Debt seeded into golaw.yml (exceptions: %d, coverage exemptions: %d, vulnerability waivers: %d), recorded in %s.",
		counts[0], counts[1], counts[2], record.Path)
}

package main

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

const (
	m2LocalEdit   = "local edit must remain\n"
	m2Head        = "HEAD"
	m2Verify      = "--verify"
	m2Quiet       = "--quiet"
	m2UpdateFlag  = "--update"
	m2EditedAsset = "golaw.mk"
)

// m2LifecycleModule is the module of the lifecycle rows: it requires the
// ruleguard dsl the rendered rules file imports, as an adopter does once it
// applies the go.mod proposal of init, so a second init proposes nothing.
const m2LifecycleModule = m2Module + "\nrequire github.com/quasilyte/go-ruleguard/dsl v0.3.23\n"

// m2LifecycleSums are the go.sum lines go mod tidy writes for that require.
// The installed pre-commit lints a staged Go file under -mod=readonly, and
// ruleguard type-checks the rules file against the dsl, which a require
// without its sums cannot load.
const m2LifecycleSums = "github.com/quasilyte/go-ruleguard/dsl v0.3.23 h1:lxjt5B6ZCiBeeNO8/oQsegE6fLeCzuMRoVWSkXC4uvY=\n" +
	"github.com/quasilyte/go-ruleguard/dsl v0.3.23/go.mod h1:KeCP03KrjuSO0H1kTuZQCWlQPulDV6YMIXmpQss17rU=\n"

func (s *suite) lifecycleM2(ctx context.Context, name string) error {
	if !slices.Contains(m2LifecycleNames(), name) {
		return fmt.Errorf("unknown M2 lifecycle case %q: %w", name, ErrAcceptance)
	}
	repo, err := s.m2LifecycleRepository(ctx, name)
	if err != nil {
		return err
	}
	switch name {
	case "init-unarmed":
		return s.m2InitUnarmed(ctx, repo)
	case "init-idempotent":
		return s.m2InitIdempotent(ctx, repo)
	case "dispatch-unarmed":
		return s.m2DispatchUnarmed(ctx, repo)
	}
	if err := s.m2LifecycleBaseline(ctx, repo); err != nil {
		return err
	}
	return s.m2ArmedLifecycle(ctx, repo, name)
}

func (s *suite) m2ArmedLifecycle(ctx context.Context, repo, name string) error {
	switch name {
	case "setup-clean":
		return s.m2SetupClean(ctx, repo)
	case "setup-idempotent":
		return s.m2SetupIdempotent(ctx, repo)
	case "setup-interrupted":
		return s.m2InterruptedSetup(ctx, repo)
	case "setup-uninstall":
		return s.m2SetupUninstall(ctx, repo)
	case "update-preserves-edit":
		return s.m2UpdatePreservesEdit(ctx, repo)
	case "upgrade-commit":
		return s.m2UpgradeCommit(ctx, repo)
	}
	if strings.HasSuffix(name, "-disabled") {
		return s.m2SwitchCase(ctx, repo, name)
	}
	return s.m2PinCase(ctx, repo, name)
}

func m2LifecycleNames() []string {
	return []string{
		"init-unarmed", "init-idempotent", "dispatch-unarmed",
		"setup-clean", "setup-idempotent", "setup-interrupted", "setup-uninstall",
		"update-preserves-edit", "upgrade-commit",
		"pre-commit-disabled", "commit-msg-disabled", "pre-push-disabled",
		"pre-commit-older", "pre-commit-equal", "pre-commit-newer",
		"commit-msg-older", "commit-msg-equal", "commit-msg-newer",
		"pre-push-older", "pre-push-equal", "pre-push-newer",
	}
}

func (s *suite) m2LifecycleRepository(ctx context.Context, name string) (string, error) {
	repo, err := s.repository(ctx, name, false)
	if err != nil {
		return "", err
	}
	if err := materialize(repo, map[string]string{moduleFile: m2LifecycleModule, "go.sum": m2LifecycleSums}); err != nil {
		return "", err
	}
	if err := s.gitSteps(ctx, repo, []string{commandInit, m2Quiet}); err != nil {
		return "", err
	}
	return repo, nil
}

func (s *suite) m2LifecycleBaseline(ctx context.Context, repo string) error {
	if err := s.m2Initialize(ctx, repo); err != nil {
		return err
	}
	return s.gitSteps(ctx, repo, []string{gitAdd, "."}, commitArgs("test: prepare lifecycle fixture"))
}

func (s *suite) m2LifecycleCommand(ctx context.Context, repo string, args ...string) result {
	return s.execute(ctx, command{program: s.binary, args: args, dir: repo, env: s.env})
}

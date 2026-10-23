package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/buildinfo"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

type initOptions struct {
	apply        bool
	update       bool
	language     string
	capabilities []string
	accept       []string
	profile      string
	pin          *policy.ProfileRef // the installed bundle --profile names, resolved once
}

// initCommand previews policy and template changes before an explicit apply
// writes and enrolls them. Git hooks remain unarmed until setup.
func (a *app) initCommand() *cobra.Command {
	var opts initOptions
	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Prepare golaw.yml and templates; without --apply print only the diff",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.initialize(cmd.Context(), cmd.OutOrStdout(), rootArg(args), opts)
		},
	}
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "write the policy file, the templates and the enrollment record")
	cmd.Flags().BoolVar(&opts.update, "update", false, "re-render the managed files golaw.yml records and move the kit pin to this binary")
	cmd.Flags().StringVar(&opts.language, "language", model.DefaultLanguage, "language of the human channel")
	cmd.Flags().StringSliceVar(&opts.capabilities, "capabilities", nil,
		"capabilities to enable: "+strings.Join(model.Capabilities(), ", "))
	cmd.Flags().StringSliceVar(&opts.accept, "accept", nil,
		"write the proposal init prints for this destination as well; repeat for several")
	cmd.Flags().StringVar(&opts.profile, "profile", "", "enroll the installed profile bundle of this name, read from the config directory")
	return cmd
}

func (a *app) initialize(ctx context.Context, out io.Writer, path string, opts initOptions) error {
	if opts.update && len(opts.accept) > 0 {
		return initFailed(fmt.Errorf("%w: --accept writes a proposal of init, while init --update keeps every local edit", model.ErrUsage))
	}
	if err := a.resolveProfile(ctx, path, &opts); err != nil {
		return initFailed(err)
	}
	if opts.update {
		if err := updateCapabilities(opts); err != nil {
			return initFailed(err)
		}
		return a.initializeUpdate(ctx, out, path, opts.apply)
	}
	root, current, data, err := a.prepareInitSnapshot(path, opts)
	if err != nil {
		return err
	}
	plan, err := a.planInit(ctx, root, current, data, opts.apply, opts.accept)
	if err != nil {
		return initFailed(err)
	}
	plan.status = a.unchangedPolicyStatus(root, opts, plan)
	return a.finishInit(ctx, out, root, plan, opts.apply)
}

func (a *app) finishInit(ctx context.Context, out io.Writer, root string, plan initPlan, apply bool) error {
	if err := printInitPolicy(out, plan); err != nil {
		return initFailed(err)
	}
	if !apply {
		return initFailed(previewInit(out, plan))
	}
	if err := applyInit(root, plan, out); err != nil {
		return initFailed(err)
	}
	grown, err := a.grownAtHead(ctx, root, plan)
	if err != nil {
		return initFailed(err)
	}
	if grown {
		return initFailed(writeLines(out, []string{enrollAfterCommitLine(plan.policy.Language)}))
	}
	return a.enrollInit(ctx, out, root, plan.data, plan.policy)
}

// enrollInit writes the enrollment record of the policy init settled on
// and, while HEAD does not hold that policy, ends with the line that asks
// for golaw.yml to be committed: until a commit holds it, it governs the
// agent hooks only, and a HEAD that cannot be read still asks.
func (a *app) enrollInit(ctx context.Context, out io.Writer, root string, data []byte, compiled *model.EffectivePolicy) error {
	if err := a.enroll(root, data, compiled, a.now()); err != nil {
		return initFailed(err)
	}
	lines := []string{"enrolled: " + root + " policy " + compiled.Digest}
	if committed, err := a.committed(ctx, root, compiled.Digest); err != nil || !committed {
		lines = append(lines, commitLine(compiled.Language))
	}
	return initFailed(writeLines(out, lines))
}

// commitLine asks the person to commit golaw.yml, in the language of the
// policy: Portuguese for pt-BR and English for any other.
func commitLine(language string) string {
	if language == model.DefaultLanguage {
		return "Faça o commit de golaw.yml para fixar a política em HEAD."
	}
	return "Commit golaw.yml to fix the policy at HEAD."
}

// prepareInit preserves reviewed settings before flags can supply defaults.
func (a *app) prepareInit(path string, opts initOptions) (string, []byte, *model.EffectivePolicy, error) {
	root, err := bootstrap.Root(path)
	if err != nil {
		return "", nil, nil, initFailed(err)
	}
	data, existing, err := initialPolicy(root, pinOf(a.buildInfo()), opts)
	if err != nil {
		return "", nil, nil, initFailed(err)
	}
	compiled, err := a.compile(data, a.now())
	if err != nil {
		if existing {
			err = fmt.Errorf("%s of %s cannot be loaded, so init neither replaces nor enrolls it: %w", bootstrap.PolicyFile, root, err)
		}
		return "", nil, nil, initFailed(err)
	}
	return root, data, compiled, nil
}

func (a *app) unchangedPolicyStatus(root string, opts initOptions, plan initPlan) string {
	if plan.current == nil || !bytes.Equal(plan.current, plan.data) {
		return ""
	}
	kept := "golaw.yml: kept as reviewed, init enrolls it and renders nothing over it"
	// Hypothetical defaults must not invalidate a policy already reviewed.
	defaults, err := renderInitialPolicy(root, pinOf(a.buildInfo()), opts)
	if err != nil {
		return kept
	}
	if plan.notice == "" {
		defaults, err = initManagedPolicy(defaults, plan.templates)
		if err != nil {
			return kept
		}
	}
	if bytes.Equal(plan.data, defaults) {
		return "golaw.yml: no changes"
	}
	return kept
}

// developmentPin is the kit version a development build writes: a released
// binary refuses it under A10 until init --update pins the release, so a
// policy born from a local build never passes for a released one.
const developmentPin = "0.0.0"

// pinOf is the kit version init pins: the exact release of the binary, or
// the development pin.
func pinOf(info buildinfo.Info) string {
	if release, ok := info.Release(); ok {
		return release
	}
	return developmentPin
}

// diffLines is what init prints for the diff of a file it renders.
func diffLines(diff string) []string {
	return strings.Split(strings.TrimSuffix(diff, "\n"), "\n")
}

// initFailed names the command in front of a failure and passes nil
// through, so every exit of initialize reads the same way.
func initFailed(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("init: %w", err)
}

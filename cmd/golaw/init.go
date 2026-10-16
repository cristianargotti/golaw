package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/buildinfo"
	"github.com/cristianargotti/golaw/internal/model"
)

// initOptions are the flags of init.
type initOptions struct {
	apply        bool
	language     string
	capabilities []string
}

// initCommand writes the policy file and the enrollment record of a
// repository (specification section 4.3, A2, A3, A24). Without --apply it
// prints the complete diff and writes nothing; with it, twice yields no
// diff. No template is written in M1 and no git hook is armed, ever.
func (a *app) initCommand() *cobra.Command {
	var opts initOptions
	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Write golaw.yml and enroll the repository; without --apply only print the diff",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.initialize(cmd.OutOrStdout(), rootArg(args), opts)
		},
	}
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "write the policy file and the enrollment record")
	cmd.Flags().StringVar(&opts.language, "language", model.DefaultLanguage, "language of the human channel")
	cmd.Flags().StringSliceVar(&opts.capabilities, "capabilities", nil,
		"capabilities to enable: "+strings.Join(model.Capabilities(), ", "))
	return cmd
}

// initialize renders the policy, prints the diff against the file in the
// checkout and, with --apply, publishes the file and enrolls the root with
// the digest of the policy it compiled.
func (a *app) initialize(out io.Writer, path string, opts initOptions) error {
	root, data, policy, err := a.prepareInit(path, opts)
	if err != nil {
		return err
	}
	current, err := bootstrap.Current(root)
	if err != nil {
		return initFailed(err)
	}
	diff := bootstrap.Diff(current, data)
	if err := writeLines(out, diffLines(diff)); err != nil {
		return initFailed(err)
	}
	if !opts.apply {
		return initFailed(writeLines(out, []string{"init: nothing written, run again with --apply"}))
	}
	if diff != "" {
		if err := bootstrap.Write(root, data); err != nil {
			return initFailed(err)
		}
	}
	if err := a.enroll(root, data, policy, a.now()); err != nil {
		return initFailed(err)
	}
	return initFailed(writeLines(out, []string{"enrolled: " + root + " policy " + policy.Digest}))
}

// prepareInit resolves the root, detects the layout, renders the policy and
// compiles it, so a policy the kit itself cannot load is never written.
func (a *app) prepareInit(path string, opts initOptions) (string, []byte, *model.EffectivePolicy, error) {
	root, err := bootstrap.Root(path)
	if err != nil {
		return "", nil, nil, initFailed(err)
	}
	layout, err := bootstrap.Detect(root)
	if err != nil {
		return "", nil, nil, initFailed(err)
	}
	in := bootstrap.Inputs{
		KitVersion:   pinOf(a.buildInfo()),
		Language:     opts.language,
		Capabilities: opts.capabilities,
		Layout:       layout,
	}
	in, err = in.Normalize()
	if err != nil {
		return "", nil, nil, initFailed(err)
	}
	data, err := bootstrap.Render(in)
	if err != nil {
		return "", nil, nil, initFailed(err)
	}
	policy, err := a.compile(data, a.now())
	if err != nil {
		return "", nil, nil, initFailed(err)
	}
	return root, data, policy, nil
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

// diffLines is what init prints for the diff: the diff itself, or one line
// saying the file already has the rendered content.
func diffLines(diff string) []string {
	if diff == "" {
		return []string{bootstrap.PolicyFile + ": no changes"}
	}
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

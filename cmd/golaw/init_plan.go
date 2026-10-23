package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"slices"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

type initPlan struct {
	current, data []byte
	templates     bootstrap.TemplatePlan
	module        install.Change
	policy        *model.EffectivePolicy
	suggestions   []bootstrap.Suggestion
	notice        string
	debt          string // the line that counts the seeded debt set, empty without --seed-debt
	status        string
	update        bool
	refusal       error
}

// initialPolicy is the policy init proposes: the reviewed file with the
// capabilities the flag adds, or a new policy rendered from the flags.
func initialPolicy(root, pin string, opts initOptions) ([]byte, bool, error) {
	current, err := bootstrap.Current(root)
	if err != nil {
		return nil, false, initOperation("read reviewed policy", err)
	}
	if current != nil {
		grown, growErr := bootstrap.GrowCapabilities(current, opts.capabilities)
		if growErr != nil {
			return nil, true, initOperation("add the capabilities", growErr)
		}
		pinned, pinErr := pinProfile(grown, opts.pin)
		return pinned, true, pinErr
	}
	data, err := renderInitialPolicy(root, pin, opts)
	return data, false, err
}

// pinProfile sets the profile line of a reviewed policy when init was asked
// to enroll one (D7), and leaves the bytes alone otherwise.
func pinProfile(current []byte, ref *policy.ProfileRef) ([]byte, error) {
	if ref == nil {
		return current, nil
	}
	pinned, err := bootstrap.SetProfile(current, *ref)
	if err != nil {
		return nil, fmt.Errorf("pin the profile: %w", err)
	}
	return pinned, nil
}

func renderInitialPolicy(root, pin string, opts initOptions) ([]byte, error) {
	layout, err := bootstrap.Detect(root)
	if err != nil {
		return nil, initOperation("detect initial layout", err)
	}
	in, err := (bootstrap.Inputs{
		KitVersion: pin, Language: opts.language, Capabilities: opts.capabilities,
		Layout: bootstrap.ProfileLayout(root, layout, opts.layout), Profile: opts.pin,
	}).Normalize()
	if err != nil {
		return nil, initOperation("normalize policy settings", err)
	}
	data, err := bootstrap.Render(in)
	return data, initOperation("render initial policy", err)
}

func (a *app) planInit(ctx context.Context, root string, current, data []byte, apply bool, accept []string) (initPlan, error) {
	module, err := readInitModule(root)
	if err != nil {
		return initPlan{}, err
	}
	templates, notice, err := a.bind().initTemplates(ctx, root, data)
	if err != nil {
		return initPlan{}, err
	}
	if len(accept) > 0 && notice != "" {
		return initPlan{}, fmt.Errorf("%w: init rendered no template, so it has no proposal to write", bootstrap.ErrAccept)
	}
	if err = templates.Accept(accept); err != nil {
		return initPlan{}, initOperation("accept proposals", err)
	}
	// A protected golaw.yml keeps its bytes, so init keeps and enrolls it
	// as M1 did and only names the records it would add.
	if current != nil && slices.Contains(templates.Ownership.Protected, bootstrap.PolicyFile) {
		templates.HoldRecords()
	} else if notice == "" {
		data, err = initManagedPolicy(data, templates)
		if err != nil {
			return initPlan{}, err
		}
	}
	return a.compileSuggestedInitPlan(root, initPlan{
		current: current, data: data, templates: templates, module: module, notice: notice,
	}, apply)
}

func (a *app) compileInitPlan(root string, plan initPlan, apply bool) (initPlan, error) {
	// A preview mutates nothing, so it prints its diff and names the refusal.
	refusal := checkInitOwnership(plan)
	if refusal != nil && apply {
		return initPlan{}, refusal
	}
	compiled, err := a.compile(plan.data, a.now())
	if err != nil {
		return initPlan{}, err
	}
	if refusal == nil {
		refusal = a.bind().checkInitEnrollment(root, plan.data)
	}
	if refusal != nil && apply {
		return initPlan{}, refusal
	}
	if err := unchangedInitPolicy(root, plan.current); err != nil {
		return initPlan{}, err
	}
	plan.policy, plan.refusal = compiled, refusal
	return plan, unchangedInitModule(root, plan.module)
}

func initManagedPolicy(data []byte, plan bootstrap.TemplatePlan) ([]byte, error) {
	config, err := policy.Decode(data)
	if err != nil {
		return nil, initOperation("decode managed policy", err)
	}
	updated, err := bootstrap.ManagedPolicy(data, config.Kit.Version, plan.Managed)
	return updated, initOperation("record managed templates", err)
}

func printInitPolicy(out io.Writer, plan initPlan) error {
	if diff := bootstrap.Diff(plan.current, plan.data); diff != "" {
		if err := writeLines(out, diffLines(diff)); err != nil {
			return err
		}
	} else if plan.status != "" {
		if err := writeLines(out, []string{plan.status}); err != nil {
			return err
		}
	}
	var lines []string
	if plan.notice != "" {
		lines = append(lines, plan.notice)
	}
	if plan.debt != "" {
		lines = append(lines, plan.debt)
	}
	if line := profileCILine(plan); line != "" {
		lines = append(lines, line)
	}
	// The update names its local edits in its notice already.
	if !plan.update {
		for _, edit := range plan.templates.LocalEdits {
			lines = append(lines, updateLocalEdit(edit, plan.templates.Language))
		}
	}
	if line := suggestionLine(plan.policy.Language, plan.policy.Capabilities, plan.suggestions); line != "" {
		lines = append(lines, line)
	}
	return writeLines(out, lines)
}

func previewInit(out io.Writer, plan initPlan) error {
	if err := bootstrap.WriteTemplateDiff(out, plan.templates); err != nil {
		return initOperation("preview templates", err)
	}
	line := "init: nothing written, run again with --apply"
	if plan.refusal != nil {
		line = "init: nothing written; --apply would refuse: " + plan.refusal.Error()
	}
	return writeLines(out, []string{line})
}

func applyInit(root string, plan initPlan, out io.Writer) error {
	if err := bootstrap.WriteTemplateDiff(out, plan.templates); err != nil {
		return initOperation("preview applied templates", err)
	}
	if err := unchangedInitModule(root, plan.module); err != nil {
		return err
	}
	if err := unchangedInitPolicy(root, plan.current); err != nil {
		return err
	}
	if err := bootstrap.ApplyTemplates(root, plan.templates, io.Discard); err != nil {
		return initOperation("apply templates", err)
	}
	if err := unchangedInitPolicy(root, plan.current); err != nil {
		return err
	}
	if !bytes.Equal(plan.current, plan.data) {
		return initOperation("write proposed policy", bootstrap.Write(root, plan.data))
	}
	return nil
}

func unchangedInitPolicy(root string, before []byte) error {
	current, err := bootstrap.Current(root)
	if err != nil {
		return initOperation("recheck policy", err)
	}
	if !bytes.Equal(current, before) || (current == nil) != (before == nil) {
		return fmt.Errorf("%w: %s", install.ErrChanged, bootstrap.PolicyFile)
	}
	return nil
}

func initOperation(operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", operation, err)
}

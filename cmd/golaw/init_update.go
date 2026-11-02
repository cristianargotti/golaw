package main

import (
	"context"
	"fmt"
	"io"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

func (a *app) initializeUpdate(ctx context.Context, out io.Writer, path string, apply bool) error {
	root, current, err := currentUpdatePolicy(path)
	if err != nil {
		return initFailed(err)
	}
	plan, err := a.planUpdate(ctx, root, current, apply)
	if err != nil {
		return initFailed(err)
	}
	return a.finishInit(ctx, out, root, plan, apply)
}

func currentUpdatePolicy(path string) (string, []byte, error) {
	root, err := bootstrap.Root(path)
	if err != nil {
		return "", nil, initOperation("resolve update root", err)
	}
	current, err := bootstrap.Current(root)
	if err != nil {
		return "", nil, initOperation("read policy to update", err)
	}
	if current == nil {
		return "", nil, fmt.Errorf("%w: golaw.yml is absent; run golaw init --apply before init --update", model.ErrUsage)
	}
	return root, current, nil
}

func (a *app) planUpdate(ctx context.Context, root string, current []byte, apply bool) (initPlan, error) {
	module, err := readInitModule(root)
	if err != nil {
		return initPlan{}, err
	}
	described, err := updateGenerate(root, current)
	if err != nil {
		return initPlan{}, err
	}
	pin := pinOf(a.buildInfo())
	templates, notice, err := a.bind().updateTemplates(ctx, root, described, pin)
	if err != nil {
		return initPlan{}, err
	}
	data, err := bootstrap.ManagedPolicy(described, pin, templates.Managed)
	if err != nil {
		return initPlan{}, initOperation("update managed policy", err)
	}
	return a.compileInitPlan(root, initPlan{
		current: current, data: data, templates: templates, module: module, notice: notice, update: true,
	}, apply)
}

// updateGenerate gives a reviewed policy that names no generate block the
// layout init detects, before the templates render, so the sqlc and codegen
// output paths of the same diff follow that layout (A49); a block the policy
// names is the person's and is kept as written, without detection. A policy
// that does not decode keeps the sentence the update gave it before M7.
func updateGenerate(root string, current []byte) ([]byte, error) {
	config, err := policy.Decode(current)
	if err != nil {
		return nil, initOperation("decode update settings", err)
	}
	if config.Generate != nil {
		return current, nil
	}
	layout, err := bootstrap.DetectGenerate(root)
	if err != nil {
		return nil, initOperation("detect the generate layout", err)
	}
	described, err := bootstrap.SetGenerate(current, layout)
	if err != nil {
		return nil, initOperation("write the generate block", err)
	}
	return described, nil
}

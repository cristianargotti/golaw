package main

import (
	"context"
	"fmt"
	"io"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/model"
)

func (a *app) initializeUpdate(ctx context.Context, out io.Writer, path string, apply bool) error {
	root, current, err := currentUpdatePolicy(path)
	if err != nil {
		return initFailed(err)
	}
	plan, err := a.planUpdate(ctx, root, current)
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

func (a *app) planUpdate(ctx context.Context, root string, current []byte) (initPlan, error) {
	module, err := readInitModule(root)
	if err != nil {
		return initPlan{}, err
	}
	pin := pinOf(a.buildInfo())
	templates, notice, err := a.bind().updateTemplates(ctx, root, current, pin)
	if err != nil {
		return initPlan{}, err
	}
	data, err := bootstrap.ManagedPolicy(current, pin, templates.Managed)
	if err != nil {
		return initPlan{}, initOperation("update managed policy", err)
	}
	return a.compileInitPlan(root, initPlan{
		current: current, data: data, templates: templates, module: module, notice: notice, update: true,
	})
}

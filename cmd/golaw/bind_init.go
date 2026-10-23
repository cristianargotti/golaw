package main

import (
	"context"
	"errors"
	"go/version"
	"strings"

	"github.com/cristianargotti/golaw"
	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/runner"
)

func (b binding) initTemplates(ctx context.Context, root string, data []byte) (bootstrap.TemplatePlan, string, error) {
	config, err := policy.Decode(data)
	if err != nil {
		return bootstrap.TemplatePlan{}, "", initOperation("decode template settings", err)
	}
	loaded, err := b.initBundle(config)
	if err != nil {
		return bootstrap.TemplatePlan{}, "", err
	}
	exclude, _ := profileTemplateRules(loaded)
	owners, err := b.initOwnership(ctx, root)
	if err != nil {
		return bootstrap.TemplatePlan{}, "", err
	}
	inputs, promoted, notice, err := b.initTemplateContext(ctx, root, config)
	if err != nil {
		return bootstrap.TemplatePlan{}, "", err
	}
	if notice != "" {
		_, ownership, filterErr := filterInitOwnership(nil, owners)
		return bootstrap.TemplatePlan{Ownership: ownership, Language: inputs.Language}, notice, filterErr
	}
	plan, err := renderInitTemplates(root, inputs, owners, promoted, exclude)
	return plan, "", err
}

func (b binding) initTemplateContext(ctx context.Context, root string, config policy.File) (assets.Context, []assets.Requirement, string, error) {
	layout, err := initLayout(root, config)
	if err != nil {
		return assets.Context{}, nil, "", err
	}
	values := initValues(config, layout)
	inputs := assets.Context{Values: values, CIBackend: "none"}
	if !layout.Go {
		return inputs, nil, moduleInstruction(values.Language, false), nil
	}
	module, err := bootstrap.ReadModule(ctx, root, b.environ())
	if err != nil && ctx.Err() == nil && errors.Is(err, bootstrap.ErrModule) {
		return inputs, nil, unreadableModule(values.Language, err), nil
	}
	if err != nil {
		return assets.Context{}, nil, "", initOperation("read the module", err)
	}
	if version.Compare("go"+module.GoVersion, "go1.24") < 0 {
		return inputs, nil, moduleInstruction(values.Language, true), nil
	}
	module, promoted, err := bootstrap.PromoteImported(root, module)
	if err != nil {
		return assets.Context{}, nil, "", initOperation("read the module imports", err)
	}
	inputs.ModulePath, inputs.GoVersion, inputs.DirectRequires = module.Path, module.GoVersion, module.Direct
	return inputs, promoted, "", nil
}

func renderInitTemplates(root string, inputs assets.Context, owners []byte, promoted []assets.Requirement, exclude []string) (bootstrap.TemplatePlan, error) {
	rendered, err := assets.RenderAllContext(golaw.FS, inputs)
	if err != nil {
		return bootstrap.TemplatePlan{}, initOperation("render templates", err)
	}
	allowed, ownership, err := filterInitOwnership(rendered, owners)
	if err != nil {
		return bootstrap.TemplatePlan{}, err
	}
	allowed, irregular, err := filterInitIrregular(root, allowed, func(string) bool { return true })
	if err != nil {
		return bootstrap.TemplatePlan{}, err
	}
	plan, err := bootstrap.PlanTemplates(root, allowed)
	plan.Language, plan.Ownership, plan.Irregular = inputs.Language, ownership, irregular
	plan.Exclude(exclude)
	if writesInheritedBaseline(plan, rendered) {
		plan.Promoted = promoted
	}
	return plan, initOperation("plan templates", err)
}

// writesInheritedBaseline reports whether the plan writes the inherited
// baseline record, the one case where the promoted line may say that the
// record lists those modules: init keeps a record the tree already holds,
// and one written by an earlier version or by hand may lack them.
func writesInheritedBaseline(plan bootstrap.TemplatePlan, rendered []assets.Rendered) bool {
	for i := range rendered {
		file := &rendered[i]
		if file.Template.Source != "templates/decisions/0001-inherited-baseline.md.tmpl" {
			continue
		}
		for _, change := range plan.Changes {
			if change.Path == file.Path {
				return true
			}
		}
	}
	return false
}

func initLayout(root string, config policy.File) (bootstrap.Layout, error) {
	var main, decisions *string
	if config.Layout != nil {
		main, decisions = config.Layout.Main, config.Layout.Decisions
	}
	layout, err := bootstrap.ResolveLayout(root, main, decisions)
	return layout, initOperation("resolve template layout", err)
}

func initValues(config policy.File, layout bootstrap.Layout) assets.Values {
	values := assets.Values{
		KitVersion: config.Kit.Version, Language: model.DefaultLanguage,
		Capabilities: config.Capabilities, Main: layout.Main, Decisions: layout.Decisions,
		Tools: make(map[string]string),
	}
	if config.Language != nil {
		values.Language = *config.Language
	}
	for _, tool := range runner.Toolchain() {
		values.Tools[tool.Name] = tool.Version
	}
	return values
}

func moduleInstruction(language string, older bool) string {
	if language == model.DefaultLanguage {
		if older {
			return "Atualize a diretiva go do go.mod para 1.24 ou mais nova antes de gerar os templates."
		}
		return "Execute go mod init <module path> antes de gerar os templates."
	}
	if older {
		return "Raise the go directive of go.mod to 1.24 or newer before generating templates."
	}
	return "Run go mod init <module path> before generating templates."
}

// unreadableModule keeps init to the policy and the enrollment of M1 when
// the go command cannot read go.mod, and names the cause on one line.
func unreadableModule(language string, err error) string {
	cause := strings.Join(strings.Fields(strings.TrimPrefix(err.Error(), bootstrap.ErrModule.Error()+": ")), " ")
	if language == model.DefaultLanguage {
		return "O comando go não leu go.mod (" + cause +
			"); coloque go no PATH ou corrija go.mod e execute golaw init de novo antes de gerar os templates."
	}
	return "The go command could not read go.mod (" + cause +
		"); put go on PATH or repair go.mod, then run golaw init again before generating templates."
}

// checkInitEnrollment refuses a weakening before any template is published.
func (b binding) checkInitEnrollment(root string, data []byte) error {
	dirs, err := b.dirs()
	if err != nil {
		return initOperation("read existing enrollment", err)
	}
	record, found, err := policy.ReadRecord(dirs.State, root)
	if err != nil || !found {
		return initOperation("read existing enrollment", err)
	}
	config, err := policy.Decode(data)
	if err != nil {
		return initOperation("decode proposed enrollment", err)
	}
	return initOperation("check proposed enrollment", policy.CheckReenrollment(record, config, schemas()))
}

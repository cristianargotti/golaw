package main

import (
	"context"
	"go/version"

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
	layout, err := initLayout(root, config)
	if err != nil {
		return bootstrap.TemplatePlan{}, "", err
	}
	values := initValues(config, layout)
	if !layout.Go {
		return bootstrap.TemplatePlan{}, moduleInstruction(values.Language, false), nil
	}
	module, err := bootstrap.ReadModule(ctx, root, b.environ())
	if err != nil {
		return bootstrap.TemplatePlan{}, "", initOperation("read the module", err)
	}
	if version.Compare("go"+module.GoVersion, "go1.24") < 0 {
		return bootstrap.TemplatePlan{}, moduleInstruction(values.Language, true), nil
	}
	values.ModulePath, values.GoVersion = module.Path, module.GoVersion
	plan, err := renderInitTemplates(root, assets.Context{
		Values: values, DirectRequires: module.Direct, CIBackend: "none",
	})
	return plan, "", err
}

func renderInitTemplates(root string, inputs assets.Context) (bootstrap.TemplatePlan, error) {
	rendered, err := assets.RenderAllContext(golaw.FS, inputs)
	if err != nil {
		return bootstrap.TemplatePlan{}, initOperation("render templates", err)
	}
	plan, err := bootstrap.PlanTemplates(root, rendered)
	plan.Language = inputs.Language
	return plan, initOperation("plan templates", err)
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

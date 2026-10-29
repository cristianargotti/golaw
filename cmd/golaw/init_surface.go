package main

import (
	"context"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/lint"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/surface"
)

// inventoryCheck is the outcome lint records for files it could not
// judge; it names no tool, so setup cannot supply its count.
const inventoryCheck = "lint.inventory"

// surfacePolicy sets the surface floor init proposes (A31): a named flag
// sets that floor, a reviewed policy without the flag keeps its bytes, and
// a fresh policy starts at strict.
func surfacePolicy(data []byte, existing bool, flag string) ([]byte, error) {
	rules := flag
	if rules == "" {
		if existing {
			return data, nil
		}
		rules = model.SurfaceStrict
	}
	floored, err := bootstrap.SetSurfaceRules(data, rules)
	return floored, initOperation("set the surface floor", err)
}

// checkSurfaceFlag refuses the flag where init would ignore it, since
// init --update keeps every reviewed line as it stands.
func checkSurfaceFlag(opts initOptions) error {
	if opts.update && opts.surfaceRules != "" {
		return fmt.Errorf("%w: --surface-rules sets the floor of a new or reviewed policy, while init --update keeps every local edit", model.ErrUsage)
	}
	return nil
}

// surfaceRulesOf is the surface floor a policy names, empty when it names
// none or does not decode.
func surfaceRulesOf(data []byte) string {
	if data == nil {
		return ""
	}
	file, err := policy.Decode(data)
	if err != nil || file.Surface == nil || file.Surface.Rules == nil {
		return ""
	}
	return *file.Surface.Rules
}

// printSurfaceDebt counts what the strict set finds in the tree when init
// is the one moving the floor to strict, so the debt is never seeded
// silently (A24). The count is advisory: anything that keeps it from
// running prints nothing and never fails init.
func (a *app) printSurfaceDebt(ctx context.Context, out io.Writer, root string, plan initPlan) error {
	if surfaceRulesOf(plan.data) != model.SurfaceStrict || surfaceRulesOf(plan.current) == model.SurfaceStrict {
		return nil
	}
	run, ok := a.strictRun(ctx, root, plan.policy)
	if !ok {
		return nil
	}
	language := plan.policy.Language
	return writeLines(out, debtLines(run, language == "" || language == model.DefaultLanguage))
}

// strictRun lints the tree with the strict set and the installed tools.
func (a *app) strictRun(ctx context.Context, root string, p *model.EffectivePolicy) (*engine.Run, bool) {
	b := a.bind()
	snapshot, err := b.snapshot(ctx, root)
	if err != nil {
		return nil, false
	}
	scope, err := surface.ScopeOf(ctx, snapshot, p)
	if err != nil {
		return nil, false
	}
	dirs, err := b.dirs()
	if err != nil {
		return nil, false
	}
	run, err := lint.Run(ctx, lint.Options{
		Root: root, Snapshot: snapshot, Policy: p, Scope: scope,
		Set: model.SurfaceStrict, Resolver: b.lintResolver(dirs), Kit: b.pin(),
	})
	return run, err == nil
}

// debtLines are the lines of the count: the findings per tool and rule,
// the reviewed floor to start from, and the tools that could not run.
func debtLines(run *engine.Run, portuguese bool) []string {
	counts := map[string]int{}
	findings := run.Findings()
	for i := range findings {
		tool, rule, _ := strings.Cut(findings[i].Rule, "/")
		counts[tool+" "+rule]++
	}
	var lines []string
	if len(findings) > 0 {
		lines = append(lines, debtHeader(len(findings), portuguese))
		for _, key := range slices.Sorted(maps.Keys(counts)) {
			lines = append(lines, fmt.Sprintf("  %s: %d", key, counts[key]))
		}
		lines = append(lines, debtAdvice(portuguese))
	}
	if tools := unmeasuredTools(run); len(tools) > 0 {
		lines = append(lines, noCountLine(strings.Join(tools, ", "), portuguese))
	}
	return lines
}

// unmeasuredTools names, sorted, the tools whose outcome left no count.
func unmeasuredTools(run *engine.Run) []string {
	var tools []string
	unmeasured := run.Unmeasured()
	for i := range unmeasured {
		if id := unmeasured[i].Spec.ID; id != inventoryCheck {
			tools = append(tools, strings.TrimPrefix(id, "lint."))
		}
	}
	slices.Sort(tools)
	return slices.Compact(tools)
}

func debtHeader(count int, portuguese bool) string {
	if portuguese {
		noun := "problemas"
		if count == 1 {
			noun = "problema"
		}
		return fmt.Sprintf("surface: o conjunto strict encontra %d %s no que o repositório já tem:", count, noun)
	}
	noun := "issues"
	if count == 1 {
		noun = "issue"
	}
	return fmt.Sprintf("surface: the strict set finds %d %s in what the repository already holds:", count, noun)
}

func debtAdvice(portuguese bool) string {
	if portuguese {
		return "Para começar pelo piso revisado, rode golaw init --surface-rules parity; o piso sobe para strict por um diff revisado."
	}
	return "To start from the reviewed floor, run golaw init --surface-rules parity; the floor rises to strict through a reviewed diff."
}

func noCountLine(tools string, portuguese bool) string {
	if portuguese {
		return "surface: sem contagem para " + tools + " (rode golaw setup --surface)"
	}
	return "surface: no count for " + tools + " (run golaw setup --surface)"
}

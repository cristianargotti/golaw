package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/golaw"
	"github.com/cristianargotti/golaw/internal/rulesdoc"
)

// rulesCommand prints the rule tables and checklists the binary embeds,
// so an agent reads one rule or one section locally in one call.
func (a *app) rulesCommand() *cobra.Command {
	var checklist string
	cmd := &cobra.Command{
		Use:   "rules [<id or family>]",
		Short: "Print a rule of docs/rules.md by id or family, or a section of docs/checklists.md",
		Long: "With no argument, one line per rule id. With an id, a family or a rule reference prefix " +
			"(commit.shape, goast, rules/4.12), every column of the matching rules. " +
			"--checklist <name> prints one section of the checklists; an unknown name lists the names.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			lines, err := rulesLines(args, checklist, cmd.Flags().Changed("checklist"))
			if err != nil {
				return fmt.Errorf("rules: %w", err)
			}
			if err := writeLines(cmd.OutOrStdout(), lines); err != nil {
				return fmt.Errorf("rules: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&checklist, "checklist", "", "print one section of docs/checklists.md by name, such as reviewer")
	return cmd
}

// rulesLines chooses the page and the form: a checklist section, the
// matching rules in full, or the index of every rule.
func rulesLines(args []string, checklist string, wantsChecklist bool) ([]string, error) {
	if wantsChecklist {
		if len(args) > 0 {
			return nil, rulesdoc.ErrTwoQueries
		}
		return checklistLines(checklist)
	}
	data, err := golaw.Docs.ReadFile("docs/rules.md")
	if err != nil {
		return nil, fmt.Errorf("read embedded rules: %w", err)
	}
	rules := rulesdoc.ParseRules(data)
	if len(args) == 0 {
		lines := make([]string, 0, len(rules))
		for _, rule := range rules {
			lines = append(lines, rule.IndexLine())
		}
		return lines, nil
	}
	kept, err := rulesdoc.Select(rules, args[0])
	if err != nil {
		return nil, fmt.Errorf("select: %w", err)
	}
	lines := make([]string, 0, len(kept))
	for _, rule := range kept {
		lines = append(lines, rule.Line())
	}
	return lines, nil
}

// checklistLines prints one section of the embedded checklists page.
func checklistLines(name string) ([]string, error) {
	data, err := golaw.Docs.ReadFile("docs/checklists.md")
	if err != nil {
		return nil, fmt.Errorf("read embedded checklists: %w", err)
	}
	section, err := rulesdoc.Find(rulesdoc.ParseSections(data), name)
	if err != nil {
		return nil, fmt.Errorf("checklist: %w", err)
	}
	return section.Lines, nil
}

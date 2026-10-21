package main

import (
	"context"
	"fmt"
	"io"
	"slices"

	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/hook/agents"
)

func withSetupLines(reports []doctor.Report, lines []doctor.Line) []doctor.Report {
	out := slices.Clone(reports)
	if len(out) > 0 {
		out[0].Lines = append(slices.Clone(out[0].Lines), lines...)
	}
	return out
}

func (a *app) doctorReports(ctx context.Context, root, scope string, repository bool) ([]doctor.Report, error) {
	reports := make([]doctor.Report, 0, len(agents.All()))
	for _, agent := range agents.All() {
		report, err := a.doctorReport(ctx, agent, root, scope, a.now())
		if err != nil {
			return nil, err
		}
		reports = append(reports, report)
	}
	// Setup refuses a directory outside git, so a setup line there would
	// name a fix that cannot work, and no policy is in force there.
	if !repository {
		return reports, nil
	}
	lines, err := a.bind().setupDoctor(ctx, root)
	if err != nil {
		return nil, err
	}
	lines = append(lines, a.bind().managedDoctor(ctx, root, a.now())...)
	lines = append(lines, a.bind().capabilitiesDoctor(ctx, root, a.now())...)
	return withSetupLines(reports, lines), nil
}

func printDoctorReports(out io.Writer, reports []doctor.Report, asJSON bool) error {
	for index, report := range reports {
		if !asJSON {
			if err := writeLines(out, []string{"agent: " + agents.All()[index].ID}); err != nil {
				return err
			}
		}
		if err := doctor.Print(out, report, asJSON); err != nil {
			return fmt.Errorf("print the report: %w", err)
		}
	}
	return nil
}

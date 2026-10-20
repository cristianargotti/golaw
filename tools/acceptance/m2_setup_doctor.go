package main

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/setup"
)

func (s *suite) m2VerifyDoctor(ctx context.Context, repo string, env []string, path string, receipt setup.Receipt) error {
	got := s.execute(ctx, command{
		program: s.binary, args: []string{"doctor", m2JSONFlag}, dir: repo, env: env,
	})
	if err := success(got); err != nil {
		return err
	}
	var doc doctor.Report
	if err := json.Unmarshal([]byte(got.stdout), &doc); err != nil {
		return fmt.Errorf("decode M2 setup doctor: %w", err)
	}
	if doc.Schema != 1 {
		return fmt.Errorf("M2 setup doctor schema differs: %w", ErrAcceptance)
	}
	for _, fact := range []string{path, receipt.HooksPath} {
		if !slices.ContainsFunc(doc.Lines, func(line doctor.Line) bool {
			return line.State == "ok" && strings.Contains(line.Key+" "+line.Detail, fact)
		}) {
			return fmt.Errorf("M2 setup doctor omits %s: %w", fact, ErrAcceptance)
		}
	}
	return m2DoctorTools(doc)
}

func m2DoctorTools(doc doctor.Report) error {
	for _, tool := range runner.Toolchain() {
		if !slices.ContainsFunc(doc.Lines, func(line doctor.Line) bool {
			content := line.Key + " " + line.Detail
			return line.State == "ok" && strings.Contains(content, tool.Name) && tool.Matches(content)
		}) {
			return fmt.Errorf("M2 setup doctor omits tool pin %s %s: %w", tool.Name, tool.Version, ErrAcceptance)
		}
	}
	return nil
}

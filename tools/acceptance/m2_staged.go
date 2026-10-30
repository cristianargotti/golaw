package main

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
	scanreport "github.com/cristianargotti/golaw/internal/report"
)

func (s *suite) stagedM2(ctx context.Context, name string) error {
	if !slices.Contains(m2StagedNames(), name) {
		return fmt.Errorf("unknown M2 staged case %q: %w", name, ErrAcceptance)
	}
	repo, err := s.m2StagedRepository(ctx, name)
	if err != nil {
		return err
	}
	before, err := s.captureM2(ctx, repo, s.env)
	if err != nil {
		return err
	}
	got := s.execute(ctx, command{
		program: s.binary, args: []string{commandScan, "--staged", "--json"},
		dir: repo, env: s.env,
	})
	after, err := s.captureM2(ctx, repo, s.env)
	if err != nil {
		return err
	}
	if before.Head != after.Head || before.Tree != after.Tree ||
		!maps.Equal(before.Files, after.Files) || !maps.Equal(before.Indexes, after.Indexes) {
		return fmt.Errorf("staged scan changed repository state: %w", ErrAcceptance)
	}
	return m2JudgeStaged(name, got)
}

func m2StagedNames() []string {
	families := []string{
		"partial", "opposite", m2Deletion, m2Rename,
		"dirty", "alternate", categoryPolicy, "length",
	}
	names := make([]string, 0, 2*len(families))
	for _, family := range families {
		names = append(names, family+"-red", family+"-near")
	}
	return names
}

func m2JudgeStaged(name string, got result) error {
	if got.err != nil {
		return success(got)
	}
	if name == "policy-red" {
		return m2PolicyAnswer(got)
	}
	if (got.code != 0 && got.code != 1) || got.stderr != "" {
		return fmt.Errorf("staged scan did not complete a decision: %w", ErrAcceptance)
	}
	var doc scanreport.Document
	if err := json.Unmarshal([]byte(got.stdout), &doc); err != nil {
		return fmt.Errorf("decode staged scan decision: %w", err)
	}
	if doc.Schema != scanreport.Schema || doc.Mode != "staged" || doc.KitVersion != m2CandidateVersion ||
		doc.Counts.Unmeasured != 0 || m2Unmeasured(doc.Checks) {
		return fmt.Errorf("staged scan identity or measurement is incomplete: %w", ErrAcceptance)
	}
	if strings.HasSuffix(name, "-near") {
		if got.code == 0 && doc.Verdict == "clean" && len(doc.Findings) == 0 &&
			doc.Counts.Findings == 0 && doc.Counts.Enforced == 0 {
			return nil
		}
		return fmt.Errorf("staged near case did not stay clean: %w", ErrAcceptance)
	}
	return m2IntroducedFinding(name, got.code, doc)
}

func m2PolicyAnswer(got result) error {
	if got.code != 2 || got.stdout != "" ||
		!strings.Contains(got.stderr, "file_lines") ||
		!strings.Contains(got.stderr, "restore") ||
		!strings.Contains(got.stderr, "unstage golaw.yml") {
		return fmt.Errorf("staged policy refusal lacks its key and repair: %w", ErrAcceptance)
	}
	return nil
}

func m2Unmeasured(checks []scanreport.Status) bool {
	return slices.ContainsFunc(checks, func(check scanreport.Status) bool {
		return check.Status == model.StatusUnmeasured.String()
	})
}

func m2IntroducedFinding(name string, code int, doc scanreport.Document) error {
	check, path := "text.dashes", "note.txt"
	switch name {
	case "length-red":
		check, path = "goast.file-length", "big.go"
	case "deletion-red", "rename-red":
		path = "new.txt"
	case "dirty-red":
		path = "dirty.txt"
	}
	if code == 1 && doc.Verdict == "fail" && doc.Counts.Enforced > 0 {
		for k := range doc.Findings {
			finding := &doc.Findings[k]
			if finding.Check == check && finding.Path == path && finding.Severity.Valid() && finding.Enforced() {
				return nil
			}
		}
	}
	return fmt.Errorf("staged scan lacks enforced %s at %s: %w", check, path, ErrAcceptance)
}

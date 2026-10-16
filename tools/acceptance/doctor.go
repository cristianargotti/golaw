package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/cristianargotti/golaw/internal/doctor"
)

const (
	categoryInstallation = "installation"
	caseDoctor           = "doctor-json"
	repoInstallation     = "installation"
	lineArtifacts        = "doctor.artifacts"
	lineBinary           = "doctor.binary"
	lineHooks            = "doctor.hooks"
	lineBuild            = "doctor.build"
)

// doctorRow replays item 2 of section 10.3: once an installed hook ran in
// the installed repository, doctor --json decodes, every artifact matches
// its receipt, the hook names the built binary, every hook blocks on
// failure and a source build is metadata, never verified (section 11 M1).
func (s *suite) doctorRow(ctx context.Context) row {
	repo := filepath.Join(s.base, dirRepos, repoInstallation)
	input, err := payload(s.root, repo, writeFixture(caseDoctor, pathNote, textSafe, wantAllow))
	if err == nil {
		err = expect(s.invoke(ctx, repo, eventPre, toolWrite, input), wantAllow)
	}
	if err != nil {
		return measured(categoryInstallation, caseDoctor, err)
	}
	got := s.execute(ctx, command{
		program: s.binary, args: []string{"doctor", "--json"}, dir: repo, env: s.env,
	})
	if err := success(got); err != nil {
		return measured(categoryInstallation, caseDoctor, err)
	}
	var report doctor.Report
	decoder := json.NewDecoder(bytes.NewReader([]byte(got.stdout)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&report); err != nil {
		return measured(categoryInstallation, caseDoctor, fmt.Errorf("decode doctor --json: %w", err))
	}
	return measured(categoryInstallation, caseDoctor, judgeDoctor(report))
}

// judgeDoctor holds every line item 2 of section 10.3 names to its code
// and requires each of them at least once.
func judgeDoctor(report doctor.Report) error {
	want := map[string]string{
		lineArtifacts: "artifact-matches",
		lineBinary:    "binary-matches",
		lineHooks:     "hook-failure-blocked",
		lineBuild:     "source-build-unverifiable",
	}
	seen := make(map[string]bool, len(want))
	for _, line := range report.Lines {
		code, judged := want[line.ID]
		if !judged {
			continue
		}
		if line.Code != code {
			return fmt.Errorf("doctor %s %s is %s (%s): %w", line.ID, line.Key, line.Code, line.Detail, ErrAcceptance)
		}
		seen[line.ID] = true
	}
	if len(seen) != len(want) {
		return fmt.Errorf("doctor --json lacks a line of section 10.3: %w", ErrAcceptance)
	}
	return nil
}

package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/cristianargotti/golaw/internal/doctor"
)

// m5Installed is what the install fake prints, so a row can judge the
// line the candidate answered.
const m5Installed = "Perfil fixture 1.2.3 instalado\n"

// m5SharedCase serves the fixture bundle, installs it, adopts a
// repository under it and reads its doctor line: every shared step a row
// composes, in order.
func m5SharedCase() m5Case {
	return m5Case{name: "shared", run: func(s *suite, ctx context.Context) error {
		source, err := s.m5Source(ctx)
		if err != nil {
			return err
		}
		got, err := s.m5Install(ctx, source, m5Tag)
		if err != nil {
			return err
		}
		if got.stdout != m5Installed {
			return fmt.Errorf("the install printed %q: %w", got.stdout, ErrAcceptance)
		}
		repo, err := s.m5Repository(ctx, "adopted", true)
		if err != nil {
			return err
		}
		return s.m5ExpectProfile(ctx, repo, "profile-installed", "version "+m5Version)
	}}
}

// TestM5SharedStepsFailWithEveryCommandTheyRun holds the frame every row
// builds on: the shared steps pass over the simulated candidate, and fail
// whichever of their commands fails, so a row composed of them never
// swallows a failure.
func TestM5SharedStepsFailWithEveryCommandTheyRun(t *testing.T) {
	t.Parallel()
	installed := doctor.Line{ID: m5DoctorProfile, Key: m5Name, State: "ok", Code: "profile-installed", Detail: "version 1.2.3, digest d"}
	m5Sweep(t, "m5-sample", m5SharedCase(), func(t *testing.T, _ *suite) []m5Reply {
		t.Helper()
		return []m5Reply{
			{prefix: []string{m5ProfileCommand, m5InstallSub}, answer: m5After(m5FakeInstall(m5Version), result{stdout: m5Installed})},
			{prefix: []string{commandInit}, answer: m5After(m5FakeInit, result{})},
			{prefix: []string{m5DoctorCommand}, answer: m5Answer(result{stdout: m5DoctorStream(t, installed)})},
		}
	})
}

// TestM5DoctorLineReadsTheFirstDocumentToo holds the decoder to every
// document of the stream: a line of the first agent's report is found as
// well as one of the second.
func TestM5DoctorLineReadsTheFirstDocumentToo(t *testing.T) {
	t.Parallel()
	got, err := m5DoctorLine(m5DoctorStream(t), "doctor.agent")
	if err != nil || got.Code != "agent" {
		t.Fatalf("line = %+v, %v", got, err)
	}
}

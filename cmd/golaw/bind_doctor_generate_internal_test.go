package main

import (
	"testing"
)

func TestGenerateDoctorNamesTheBinaryNoSetupBuilt(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	root := repository(t, true)
	lines := b.generateDoctor(t.Context(), root, now())
	if len(lines) != 1 || lines[0].Key != "golangci-lint" || lines[0].Code != "gen-tool-missing" || lines[0].State != "missing" {
		t.Fatalf("lines = %+v", lines)
	}
}

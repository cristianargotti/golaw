package bootstrap_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/install"
)

// ownershipCase is one Makefile the templates meet: its bytes before, the
// ownership class of the template, and what the plan writes and proposes.
type ownershipCase struct {
	name, before, want, proposal string
	ownership                    assets.Ownership
	writes, proposals, managed   int
}

func TestTemplateApplicationPreservesOwnershipClasses(t *testing.T) {
	t.Parallel()
	for _, tc := range []ownershipCase{
		{"new kit", "", "include golaw.mk\n", "", assets.OwnedByKit, 1, 0, 1},
		{"matching kit", "include golaw.mk\n", "include golaw.mk\n", "", assets.OwnedByKit, 0, 0, 1},
		{"local kit", "local\n", "local\n", "include golaw.mk\n", assets.OwnedByKit, 0, 1, 0},
		{"new merged", "", "include golaw.mk\n", "", assets.OwnedMerged, 1, 0, 0},
		{
			"existing merged", "all:\n\tgo build ./...\n", "all:\n\tgo build ./...\n",
			"all:\n\tgo build ./...\ninclude golaw.mk\n", assets.OwnedMerged, 0, 1, 0,
		},
		{
			"already merged", "all:\n\tgo build ./...\ninclude golaw.mk\n",
			"all:\n\tgo build ./...\ninclude golaw.mk\n", "", assets.OwnedMerged, 0, 0, 0,
		},
		{"new developer", "", "include golaw.mk\n", "", assets.OwnedByDeveloper, 1, 0, 0},
		{"existing developer", "local\n", "local\n", "", assets.OwnedByDeveloper, 0, 0, 0},
		{"matching developer", "include golaw.mk\n", "include golaw.mk\n", "", assets.OwnedByDeveloper, 0, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			path := filepath.Join(root, "Makefile")
			writeBefore(t, path, tc.before)
			plan := requireTemplatePlan(t, root, tc)
			var out bytes.Buffer
			if err := bootstrap.ApplyTemplates(root, plan, &out); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != tc.want {
				t.Fatalf("file = %q, %v", got, err)
			}
			requireTemplateOutput(t, tc, out.String())
		})
	}
}

// writeBefore writes the bytes the file of a row holds before init, when
// it holds any.
func writeBefore(t *testing.T, path, before string) {
	t.Helper()
	if before == "" {
		return
	}
	if err := os.WriteFile(path, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
}

// requireTemplatePlan plans the Makefile template of a row and requires
// its writes, proposals, managed entries and proposed bytes.
func requireTemplatePlan(t *testing.T, root string, tc ownershipCase) bootstrap.TemplatePlan {
	t.Helper()
	data := []byte("include golaw.mk\n")
	rendered := []assets.Rendered{{
		Template: assets.Template{
			Source:      "templates/Makefile.tmpl",
			Destination: "Makefile", Ownership: tc.ownership,
		},
		Path: "Makefile", Data: data, SHA256: install.Digest(data),
	}}
	plan, err := bootstrap.PlanTemplates(root, rendered)
	if err != nil || len(plan.Changes) != tc.writes ||
		len(plan.Proposed) != tc.proposals || len(plan.Managed) != tc.managed {
		t.Fatalf("plan = %+v, %v", plan, err)
	}
	if tc.proposals != 0 && string(plan.Proposed[0].After) != tc.proposal {
		t.Fatalf("proposal = %q", plan.Proposed[0].After)
	}
	return plan
}

// requireTemplateOutput requires the proposal header for a proposal, no
// write header without a write, and silence for an unchanged file.
func requireTemplateOutput(t *testing.T, tc ownershipCase, out string) {
	t.Helper()
	if tc.proposals != 0 &&
		!strings.Contains(out, "Propostas sem gravação:\n") {
		t.Fatalf("proposal header missing: %q", out)
	}
	if tc.writes == 0 && strings.Contains(out, "Gravações:\n") {
		t.Fatalf("proposal was printed as a write: %q", out)
	}
	if tc.writes == 0 && tc.proposals == 0 && out != "" {
		t.Fatalf("unchanged file produced output: %q", out)
	}
}

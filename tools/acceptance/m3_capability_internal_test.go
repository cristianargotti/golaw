package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	scanreport "github.com/cristianargotti/golaw/internal/report"
)

func capabilityDocument(check, status string, finding bool) scanreport.Document {
	doc := scanreport.Document{Checks: []scanreport.Status{{ID: check, Status: status, Reason: "capability image is not enabled"}}}
	if finding {
		doc.Findings = []model.Finding{{Check: check, Severity: model.SeverityDeny}}
	}
	return doc
}

func TestM3CapabilityNamesAreTheFourCombinations(t *testing.T) {
	t.Parallel()
	if !slices.Equal(m3CapabilityNames(), []string{"api", "db", "hexagonal", "image"}) {
		t.Fatalf("names = %q", m3CapabilityNames())
	}
	if m3CapabilityCases()["hexagonal"].conditioned || !m3CapabilityCases()["image"].conditioned {
		t.Fatal("hexagonal shapes a rule inside a check; image conditions a check")
	}
}

func TestM3JudgeCapability(t *testing.T) {
	t.Parallel()
	image := m3CapabilityCases()["image"]
	hexagonal := m3CapabilityCases()["hexagonal"]
	for _, tc := range []struct {
		name          string
		c             m3CapabilityCase
		with, without scanreport.Document
		fail          bool
	}{
		{"near conditioned check", image, capabilityDocument("policy.image-digest", "measured", true), capabilityDocument("policy.image-digest", "not-applicable", false), false},
		{"red applies without the capability", image, capabilityDocument("policy.image-digest", "measured", true), capabilityDocument("policy.image-digest", "measured", true), true},
		{"red misses with the capability", image, capabilityDocument("policy.image-digest", "measured", false), capabilityDocument("policy.image-digest", "not-applicable", false), true},
		{"near shaped rule", hexagonal, capabilityDocument("policy.golangci", "measured", true), capabilityDocument("policy.golangci", "measured", false), false},
		{"red shaped rule found without", hexagonal, capabilityDocument("policy.golangci", "measured", true), capabilityDocument("policy.golangci", "measured", true), true},
	} {
		name := "image"
		if tc.c.checks[0] == "policy.golangci" {
			name = "hexagonal"
		}
		if err := m3JudgeCapability(name, tc.c, tc.with, tc.without); (err != nil) != tc.fail {
			t.Fatalf("%s: %v", tc.name, err)
		}
	}
}

func TestM3CapabilityRowRendersBothTrees(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	var calls [][]string
	service := m3ServiceAnswer(t, s, &calls)
	s.execute = newM2Script(t, s, func(ctx context.Context, c command) (m2Reply, bool) {
		if c.program != s.binary || len(c.args) == 0 || c.args[0] != commandScan {
			return service(ctx, c)
		}
		data, err := os.ReadFile(filepath.Join(c.dir, "Dockerfile"))
		if err != nil {
			t.Fatal(err)
		}
		status, finding := "measured", ""
		switch {
		case filepath.Base(c.dir) == "without":
			status = "not-applicable"
		case !strings.Contains(string(data), "@sha256:"):
			finding = `"findings":[{"check":"policy.image-digest","severity":"deny"}],`
		}
		doc := `{"schema":1,` + finding + `"checks":[{"id":"policy.image-digest","status":"` + status +
			`","reason":"capability image is not enabled"}]}`
		return m2Reply{got: result{stdout: doc}, target: true}, true
	}).Execute
	if err := s.capabilityM3(t.Context(), "image"); err != nil {
		t.Fatal(err)
	}
	if len(calls) < 2 || !slices.Equal(calls[0], []string{"init", "--apply", "--capabilities", "go,image"}) {
		t.Fatalf("calls = %q", calls)
	}
	if err := s.capabilityM3(t.Context(), "absent"); err == nil {
		t.Fatal("an unknown capability must fail")
	}
}

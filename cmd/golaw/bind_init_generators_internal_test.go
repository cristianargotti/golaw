package main

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/runner"
)

func TestInitValuesPinTheGenerators(t *testing.T) {
	t.Parallel()
	got := initValues(policy.File{Kit: policy.Kit{Version: "0.3.0"}}, bootstrap.Layout{}).Tools
	for _, tool := range runner.Generators() {
		if got[tool.Name] != tool.Version {
			t.Errorf("Tools[%s] = %q, want %q", tool.Name, got[tool.Name], tool.Version)
		}
	}
	for _, tool := range runner.Toolchain() {
		if got[tool.Name] != tool.Version {
			t.Errorf("Tools[%s] = %q, want %q", tool.Name, got[tool.Name], tool.Version)
		}
	}
}

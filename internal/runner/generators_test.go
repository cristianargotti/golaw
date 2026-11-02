package runner_test

import (
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/runner"
)

func TestGeneratorsArePinnedApartFromTheRungToolchain(t *testing.T) {
	t.Parallel()
	want := []runner.Tool{
		{Name: "goverter", Module: "github.com/jmattheis/goverter/cmd/goverter", Version: "1.11.1"},
		{Name: "oapi-codegen", Module: "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen", Version: "2.8.0"},
		{Name: "sqlc", Module: "github.com/sqlc-dev/sqlc/cmd/sqlc", Version: "1.31.1"},
	}
	got := runner.Generators()
	if !slices.EqualFunc(got, want, func(a, b runner.Tool) bool {
		return a.Name == b.Name && a.Module == b.Module && a.Version == b.Version && len(a.Probe) > 0
	}) {
		t.Errorf("Generators() = %+v", got)
	}
	for _, tool := range runner.Toolchain() {
		if slices.ContainsFunc(want, func(g runner.Tool) bool { return g.Name == tool.Name }) {
			t.Errorf("%s joined the rung toolchain, which setup verifies on every machine", tool.Name)
		}
	}
}

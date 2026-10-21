package assets_test

import (
	"testing"

	"github.com/cristianargotti/golaw"
	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

func TestBaselineCoversApplicationAndRenderedTools(t *testing.T) {
	t.Parallel()
	for _, directory := range []string{"docs/adr", "docs/decisions"} {
		t.Run(directory, func(t *testing.T) {
			t.Parallel()
			context := assetContext()
			context.Decisions = directory
			context.DirectRequires = []assets.Requirement{
				{Path: "example.test/library", Version: "v1.0.0"},
			}
			rendered, err := assets.RenderAllContext(golaw.FS, context)
			if err != nil {
				t.Fatal(err)
			}
			files := make(map[string][]byte)
			for _, file := range rendered {
				files[file.Path] = file.Data
			}
			module := "module example.test/service\nrequire example.test/library v1.0.0\n"
			files["go.mod"] = append([]byte(module), files["go.mod"]...)
			tree := proof.NewTree(files)
			policy := proof.Policy()
			policy.Layout.Decisions = directory
			result, err := policyfiles.DependencyRecords().Run(t.Context(), model.Input{
				Mode: model.ModeScan, Policy: policy, Snapshot: tree, Targets: tree.Files(),
			})
			if err != nil || result.Status.Kind != model.StatusMeasured ||
				len(result.Findings) != 0 {
				t.Fatalf("rendered records = %+v, error %v", result, err)
			}
		})
	}
}

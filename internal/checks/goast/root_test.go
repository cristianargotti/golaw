package goast_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/goast"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// TestMainPackagesAreCompositionRoots proves that a main package outside
// cmd/ (a tool under tools/) may read the environment and create a context,
// while a library package in the same tree may not.
func TestMainPackagesAreCompositionRoots(t *testing.T) {
	t.Parallel()
	tree := proof.NewTree(map[string][]byte{
		"tools/probe/main.go": []byte("package main\n\nimport (\n\t\"context\"\n\t\"os\"\n)\n\nfunc main() {\n\t_ = os.Getenv(\"HOME\")\n\t_ = context.Background()\n}\n"),
		"internal/app/run.go": []byte("package app\n\nimport (\n\t\"context\"\n\t\"os\"\n)\n\nfunc Run() {\n\t_ = os.Getenv(\"HOME\")\n\t_ = context.Background()\n}\n"),
	})
	in := model.Input{Mode: model.ModeScan, Policy: proof.Policy(), Snapshot: tree, Targets: tree.Files()}
	for _, c := range []model.Check{goast.EnvRead(), goast.BareContext()} {
		res, err := c.Run(t.Context(), in)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Findings) != 1 || res.Findings[0].Path != "internal/app/run.go" {
			t.Fatalf("%s findings = %+v", c.Spec().ID, res.Findings)
		}
	}
}

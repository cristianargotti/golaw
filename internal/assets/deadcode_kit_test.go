package assets_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/runner"
)

func TestKitUsesTheRenderedReachabilityAsset(t *testing.T) {
	t.Parallel()
	want := renderedAsset(t, "test/deadcode/reachability_test.go", assetContext())
	got, err := os.ReadFile(filepath.Join("..", "..", want.Path))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want.Data) {
		t.Fatal("kit reachability test differs from its rendered asset")
	}
}

// TestKitProductionExportsHaveConsumers runs the kit's copy of the asset
// over the kit itself, as its make deadcode does: every exported function
// under internal/ of a package production code imports has a production
// consumer.
func TestKitProductionExportsHaveConsumers(t *testing.T) {
	t.Parallel()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	out, err := runner.Run(t.Context(), runner.Command{
		Name: "go", Dir: root, Env: append((&exec.Cmd{}).Environ(), "GOFLAGS=", "GOWORK=off"),
		Args: []string{"test", "-tags=deadcode", "-count=1", "-run", "^TestProductionExportsHaveConsumers$", "./test/deadcode/"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Code != 0 {
		t.Fatalf("the kit's production exports without a consumer:\n%s%s", out.Stdout, out.Stderr)
	}
}

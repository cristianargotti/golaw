package assets_test

import (
	"strings"
	"testing"
)

func TestDeadcodeAssetFitsTheTightenedFileLimit(t *testing.T) {
	t.Parallel()
	asset := renderedAsset(t, "test/deadcode/reachability_test.go", assetContext())
	lines := strings.Count(string(asset.Data), "\n")
	if len(asset.Data) > 0 && asset.Data[len(asset.Data)-1] != '\n' {
		lines++
	}
	if lines > 200 {
		t.Fatalf("rendered deadcode asset has %d lines, exceeding the tightened file limit of 200", lines)
	}
}
